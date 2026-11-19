# Kubernetes hardening for a gitops service repo

Status: draft, 2026-11-19, #124. Nothing in this file is tested yet; the proof plan is in section 8.
Related: [kubernetes-access.md](kubernetes-access.md) (how an agent reads a cluster),
[platform-engineer-usecases.md](platform-engineer-usecases.md), attack path P37 in
[../attack-paths.md](../attack-paths.md).

## 1. Purpose

An agent writes to a gitops repo through merge requests. A human merges. The risk that is left:
a change that looks like a config fix makes a lower environment use live data (P37), or gives
the service more rights than its stage should have. Review alone does not catch that reliably.

This file lists the guards on the platform side that make such a change fail, for one real
example. The rule behind all of them:

> A stage can only reach what belongs to that stage, and the files that say so are not in the
> part of the repo the agent may change.

## 2. Example

Service `shop`, stages `dev`, `qa`, `live`, repo layout as in the gitops and Kargo strategy:

```text
shop-argocd/
  app.yaml                      # root app-of-apps            control file
  <stage>/
    project.yaml                # stage AppProject            control file
    shop/
      app.yaml                  # stage Application           control file (Kargo owns targetRevision)
      values.yaml               # stage Helm values           agent may change
      extra-manifests/          # resources for the cluster   agent may change
    kargo/stage.yaml            # Kargo Stage                 control file
  kargo/**                      # promotion policy, checks    control file
```

Namespaces: `shop-dev`, `shop-qa`, `shop-live`. `live` runs on its own cluster where possible.

Control files are changed by humans only. Namespace, admission policies, storage classes and
secret stores are **not** in this repo at all; they live in the platform repo, which the agent
has no workspace for.

## 3. Guards by layer

| Layer | Guard | Stops |
|---|---|---|
| GitLab | `main` protected, no direct push; merge by a human only | agent deploying on its own |
| GitLab | `CODEOWNERS` with required approval on the control files (needs Premium) | change to AppProject, Application, Kargo files passing as a normal review |
| zero | push gate rejects pushes that touch control files (not built) | the same, without Premium |
| CI | merge request pipeline renders every stage and runs the admission policies of section 5 against the result | forbidden change found before the merge, not after |
| Argo CD | one AppProject per stage: one source repo, one chart repo, one destination namespace, no cluster-scoped kinds, no RBAC kinds | PersistentVolume, ClusterRole, RoleBinding, other namespace, other chart |
| Namespace | Pod Security Admission `restricted` | inline `nfs` and `hostPath` volumes, privileged pods, host namespaces |
| Namespace | admission policy on PVCs and pods (section 5) | binding an existing volume, cross-namespace data source, inline CSI volume |
| Namespace | NetworkPolicy: egress denied by default, allowed to the stage's own backends | qa talking to the live database or the live namespace |
| Secrets | one SecretStore per namespace, scoped to the stage's path in the secret backend; no ClusterSecretStore | qa reading a live secret by reference |
| Storage | one StorageClass per stage (own pool, export or account); live on its own cluster | same backing storage reachable from two stages |
| Backends | one credential per stage for database and buckets | live host name in qa values being enough to read live data |
| Kargo | promotion to `live` waits for an operator; verification on every stage | unattended roll-out of an agent's change to live |
| zero | agent reads with a session ServiceAccount, roles `zero-k8s-*`, no Secrets, no exec | agent reading more than logs, objects and config |

## 4. Argo CD: AppProject per stage

`qa/project.yaml`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: shop-qa
  namespace: argocd
spec:
  sourceRepos:
    - https://gitlab.com/corewire/k8s/argocd/shop-argocd.git
    - registry.example.com/charts/shop
  destinations:
    - server: https://qa.example.com:6443
      namespace: shop-qa
  clusterResourceWhitelist: []          # no cluster-scoped kinds: no PersistentVolume, no ClusterRole
  namespaceResourceBlacklist:
    - {group: rbac.authorization.k8s.io, kind: Role}
    - {group: rbac.authorization.k8s.io, kind: RoleBinding}
    - {group: "", kind: ServiceAccount}             # if the chart ships its own, allow it and block token Secrets instead
    - {group: external-secrets.io, kind: SecretStore}
    - {group: networking.k8s.io, kind: NetworkPolicy}
    - {group: "", kind: ResourceQuota}
    - {group: "", kind: LimitRange}
```

The namespace itself, its labels, the NetworkPolicy and the SecretStore come from the platform
repo, so the stage Application cannot change them.

## 5. Namespace and admission

Namespace (platform repo):

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: shop-qa
  labels:
    stage: qa
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: latest
```

`restricted` allows these volume types only: `configMap`, `csi`, `downwardAPI`, `emptyDir`,
`ephemeral`, `persistentVolumeClaim`, `projected`, `secret`. The rest of the storage paths is
closed by an admission policy (Kubernetes 1.30 or newer):

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: stage-storage
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["persistentvolumeclaims", "pods"]
  validations:
    - expression: >-
        object.kind != 'PersistentVolumeClaim' ||
        !has(object.spec.volumeName) || object.spec.volumeName == ''
      message: a PVC may not name an existing volume
    - expression: >-
        object.kind != 'PersistentVolumeClaim' ||
        !has(object.spec.dataSourceRef) || !has(object.spec.dataSourceRef.namespace)
      message: a PVC may not take its data from another namespace
    - expression: >-
        object.kind != 'PersistentVolumeClaim' ||
        (has(object.spec.storageClassName) &&
         object.spec.storageClassName == namespaceObject.metadata.labels['stage'])
      message: a PVC must use the storage class of its stage
    - expression: >-
        object.kind != 'Pod' || !has(object.spec.volumes) ||
        object.spec.volumes.all(v, !has(v.csi))
      message: inline CSI volumes are not allowed
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: stage-storage
spec:
  policyName: stage-storage
  validationActions: [Deny]
  matchResources:
    namespaceSelector:
      matchExpressions:
        - {key: stage, operator: Exists}
```

The inline CSI rule also blocks the Secrets Store CSI driver. A cluster that uses it needs an
exception by driver name.

Network (platform repo). A NetworkPolicy does not cover volume mounts, the node does those; it
covers the database and bucket case:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: stage-egress
  namespace: shop-qa
spec:
  podSelector: {}
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: kube-system}}
      ports: [{protocol: UDP, port: 53}]
    - to:
        - ipBlock: {cidr: 10.20.2.15/32}      # the qa database, nothing else
      ports: [{protocol: TCP, port: 5432}]
```

Secrets (platform repo): one store per namespace, bound to the stage's path.

```yaml
apiVersion: external-secrets.io/v1
kind: SecretStore
metadata:
  name: stage
  namespace: shop-qa
spec:
  provider:
    vault:
      server: https://vault.example.com
      path: kv
      auth:
        kubernetes:
          mountPath: kubernetes-qa
          role: shop-qa                      # Vault policy: read kv/shop/qa/* only
```

The service repo may hold `ExternalSecret` objects in `extra-manifests/`; they can only name
this store, and the store can only read the stage's own path.

## 6. P37 against these guards

"Make qa use live data" by a change in `qa/shop/values.yaml` or `qa/shop/extra-manifests/`:

| Attempt | Result |
|---|---|
| inline `nfs` or `hostPath` volume with live's export | pod rejected by Pod Security Admission |
| inline CSI volume with live's volume handle | pod rejected by `stage-storage` |
| new PersistentVolume for live's backing volume | Argo CD refuses the kind (AppProject) |
| PVC with `volumeName` of an existing volume | rejected by `stage-storage` |
| PVC cloned from live's PVC or snapshot | other namespace: rejected; other cluster: not reachable |
| PVC with live's storage class | rejected by `stage-storage` |
| `ExternalSecret` that names a live secret | store cannot read the path |
| live database host in values | connection blocked by NetworkPolicy; qa credential is not valid on live |
| live bucket name in values | qa credential has no rights on it |
| change to `qa/project.yaml` to allow any of the above | control file: owner approval, push gate |

## 7. Not covered

- Data that is shared on purpose (one database for qa and live, a shared bucket). No policy
  helps; the stages must be separated first.
- What the chart itself allows through values (other image, more replicas, debug flags).
  Review and the chart's own schema cover that.
- A reviewer who approves a change to a control file.
- A cluster older than 1.30 (no ValidatingAdmissionPolicy); Kyverno or Gatekeeper can hold the
  same rules.
- Logs of the qa service that contain personal data of testers or copied live data.

## 8. Proof plan

On the kind cluster `zero-k8s` (tests/k8s), two namespaces as qa and live:

1. Apply namespace labels and `stage-storage`; run every storage attempt of section 6, expect
   the stated rejection.
2. NetworkPolicy: needs a CNI that enforces it (kind's default does since v0.24, to be checked).
3. AppProject and SecretStore rows need Argo CD and External Secrets; larger setup, ask first.
4. `zero doctor` check: namespaces that a policy grants access to carry the labels and the
   bindings. Not designed yet.
