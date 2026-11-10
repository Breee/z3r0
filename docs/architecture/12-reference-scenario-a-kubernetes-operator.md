## 12. Reference scenario A — Kubernetes operator

### Goal

> I operate Kubernetes. I need to investigate incidents, modify a GitOps repository, and sometimes perform live cluster operations. My agent should do only what I explicitly allow, in a safe way.

Assume:

```text
GitOps repository:
gitlab.com/acme/platform-gitops

Cluster:
prod-eu

Allowed namespace:
payments
```

### 12.1 Session starts read-oriented

```text
Operator
  │
  │ "Investigate why payments-api is unhealthy"
  ▼
Agent
  │
  ▼
SPIRE / session identity
```

Initial capabilities:

```text
git repository read
k8s.resource.read
k8s.pod.logs

cluster = prod-eu
namespace = payments
```

Not granted initially:

```text
k8s.pod.exec
k8s.resource.delete
k8s.resource.patch
k8s.secret.read
k8s.rbac.modify
```

### 12.2 Cluster investigation

Agent may run:

```bash
kubectl get pods -n payments
kubectl describe pod ...
kubectl logs ...
kubectl get events ...
```

Flow:

```text
Agent
  ↓
OpenShell sandbox
  ↓
kubectl
  ↓
authorized credential helper
  ↓
OpenBao / short-lived cluster identity
  ↓
Kubernetes API
  ↓
Kubernetes RBAC
  ↓
Admission / policy
```

The cluster independently enforces permissions. A bug in local policy must not produce cluster-admin access.

### 12.3 Production is read-only to normal agents

Normal agent identities do **not** receive production mutation capability. They may inspect production for diagnosis, but fixes flow through normal delivery controls.

Production diagnostic capabilities may include:

```text
k8s.resource.read
k8s.pod.logs
k8s.events.read
metrics.read
```

Explicitly excluded from normal agent identities:

```text
k8s.resource.create
k8s.resource.patch
k8s.resource.delete
k8s.pod.exec
k8s.workload.restart
k8s.secret.read
k8s.rbac.modify
k8s.crd.modify
```

`read` does not imply secret access. `k8s.secret.read` remains a distinct capability and is absent from normal agents.

### 12.4 Preferred production change path: GitOps

The agent diagnoses production, proposes a fix, validates it in DEV/QA, and uses normal GitOps/CI controls for promotion.

```text
Production problem
        ↓
Agent investigates PROD read-only
        ↓
Agent identifies fix
        ↓
feature/fix branch in GitOps repo
        ↓
DEV / QA validation
        ↓
CI / policy / tests
        ↓
MR
        ↓
human review / protected merge
        ↓
GitOps controller
        ↓
production
```

Core principle:

> **Agents investigate production. Humans and controlled delivery systems change production.**

### 12.5 DEV/QA mutation and exceptional production automation

Agents may receive write capabilities in explicitly assigned DEV/QA environments and branches. Examples:

```text
k8s.resource.patch in dev/qa namespace
k8s.workload.restart in qa
k8s.pod.exec in dev/qa
git.push feature/*
```

These remain bounded by namespace/resource policy, short-lived credentials, Kubernetes RBAC, admission, network policy, and audit.

If an enterprise later needs production remediation automation, it should be implemented as a **separate narrowly scoped system/workload identity**, not by upgrading the normal coding/operator agent. Break-glass remains a human-controlled mechanism with explicit reason, short TTL, and enhanced audit.

### 12.6 Kubernetes-specific attack paths

| Attack | Expected control |
|---|---|
| Prompt injection in pod logs tells agent to read Secrets | `k8s.secret.read` absent → deny |
| Malicious log tells agent to delete a PVC | destructive capability absent or approval required |
| Agent obtains kubeconfig | no long-lived kubeconfig secret exists in workspace |
| Agent tries another namespace | cluster + namespace capability and RBAC deny |
| Agent changes production manifest maliciously | feature branch + CI + MR approval + GitOps controls |
| Agent attempts production `kubectl exec` | no production exec capability for normal agents → deny |
| Agent attempts DEV/QA exec outside assigned scope | capability + RBAC + namespace policy → deny |

---

