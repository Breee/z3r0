# Kubernetes access for the agent

Status: draft for review (#94). Nothing here is built. 🟡 = from documentation or memory, not tested by us.

Belongs to [platform-engineer-usecases.md](platform-engineer-usecases.md) (tasks T7–T10, gap G1, G2, G7, G9) and [identity-broker-usecases.md](identity-broker-usecases.md) UC2.

## 1. Question

The human already has cluster access in some form. The agent needs read access for the basic tasks (pods, events, logs, rollout state). Which credential does the sandbox use, who issues it, and what does the audit log show afterwards?

## 2. How people reach their clusters today

| # | Method | What lies on the laptop | Lifetime | Name in the Kubernetes audit log | Revoke |
|---|---|---|---|---|---|
| M1 | Client certificate in kubeconfig (kind, kubeadm `admin.conf`, k3s) | private key, in the file | months to years | certificate CN/O, often group `system:masters` (bypasses RBAC) | not possible per certificate 🟡; only by rotating the cluster CA |
| M2 | Static ServiceAccount token in kubeconfig (Secret-based) | token, in the file | never expires | `system:serviceaccount:<ns>:<name>`, shared by everyone who has the file | delete the Secret |
| M3 | OIDC with kubelogin (`kubectl oidc-login`, exec plugin; Keycloak, Dex, Entra, GitLab) | refresh token in `~/.kube/cache/oidc-login` 🟡 | ID token minutes to 1 h, refresh token days | the human (claim, e.g. e-mail) + groups | at the identity provider; ID token lives until expiry |
| M4 | AWS EKS (`aws eks get-token`, exec plugin) after `aws sso login` | SSO session in `~/.aws/sso/cache` | cluster token 15 min 🟡, SSO session 8–12 h | IAM role ARN + session name; also CloudTrail | end the SSO session, change the access entry |
| M5 | GKE (`gke-gcloud-auth-plugin`), AKS (`kubelogin`) | gcloud / Entra refresh token | access token about 1 h | cloud identity of the human | at the cloud identity provider |
| M6 | Access proxy (Teleport, Tailscale operator, Rancher, OpenShift OAuth) | proxy session or short-lived certificate | hours | the human, as mapped by the proxy | at the proxy |

What all six have in common:

- The credential is the **human's identity with the human's rights**. Platform engineers usually hold write or admin rights.
- M3–M6 are short-lived only on the surface. Behind them is a long-lived login (refresh token, SSO cache) that can mint new cluster tokens for days without the human.
- M1 and M2 are long-lived, hard to revoke and say little about who acted. M1 with `system:masters` cannot be limited at all.

## 3. Requirements

| # | Requirement | Why |
|---|---|---|
| K1 | No credential of the human enters the sandbox: no kubeconfig, no refresh token, no SSO cache, no exec plugin | a corrupted agent would be the human, with write rights, for days |
| K2 | The agent has its own identity in the cluster | audit log separates human and agent |
| K3 | Rights are read-only without Secrets, and the **cluster** enforces this (RBAC), not zero | a bug in zero must not become a write in the cluster |
| K4 | Agent ≤ human: the human can only hand out what the cluster admin allowed for them | Z-INV: sandbox never exceeds its human |
| K5 | Short-lived (10 min to 1 h), bound to one session, revocable per session | stolen token is of little use |
| K6 | Works with every login method M1–M6 without extra code per method | not hard to use |
| K7 | Developer side: one policy entry, no new login. Cluster side: one manifest, applied once by an admin | not hard to set up |
| K8 | One chain of evidence: human → session → token → request | "clear audit" |

## 4. Options

| | Option | K1 | K2 | K3 | K4 | K5 | K6 | Verdict |
|---|---|---|---|---|---|---|---|---|
| A | Copy the kubeconfig (or mount `~/.kube`) into the sandbox | no | no | no | equal | no | yes | never |
| B | Host proxy with impersonation: zero-cp sends each request with the human's credential and `Impersonate-User: zero-agent` | yes | yes (audit shows human and impersonated user) | partly | yes (`impersonate` right needed) | per request | yes | second choice: every agent request passes a process that holds the human's rights; a proxy bug is a write as the human |
| C | **TokenRequest**: zero-cp uses the human's login once to request a short-lived token for a read-only ServiceAccount; the sandbox only ever uses that token | yes | yes | yes | yes (`create serviceaccounts/token` right needed) | yes | yes | **recommended** |
| D | Cloud-native second identity (EKS: separate IAM role with view access entry; GKE: service account impersonation) | yes | yes | yes | yes | yes | no, code per cloud | later, where the cloud audit trail is required |
| E | zero-cp in the cluster as read proxy, sandbox gets no cluster credential | yes | yes | partly | by zero | yes | yes | for cluster deployments of zero, not for the laptop |

## 5. Recommended design (option C)

### Cluster side, once, by an admin

Per namespace the agent may read (or cluster-wide for a platform team):

1. ServiceAccount `zero-agent`.
2. Role `zero-read`: `get`, `list`, `watch` on pods, `pods/log`, events, services, endpoints, configmaps (see flaw KF4), deployments, replicasets, statefulsets, daemonsets, jobs, cronjobs, ingresses, horizontalpodautoscalers; optional CRDs (Argo CD `applications`, Kargo `stages`). **Not**: secrets, `pods/exec`, `pods/attach`, `pods/portforward`, any `*/proxy` subresource (these are reachable with GET and lead out of read-only), `nodes/proxy`, RBAC objects, any write verb.
3. RoleBinding `zero-read` → `zero-agent`.
4. Role `zero-token`: `create` on `serviceaccounts/token` with `resourceNames: [zero-agent]`, plus `create`/`delete` on the session Secrets (see below), bound to the humans or groups that may use an agent there.

Step 4 is the admin's decision "who may hand read access to an agent" (K4). zero ships the manifest (`zero k8s manifest -n <ns>`), the admin reviews and applies it, GitOps or by hand.

### Developer side

```yaml
# .zero/policy.yaml
capabilities: [k8s.read]
kubernetes:
  - context: dev          # name in the human's kubeconfig
    namespaces: [team-a, team-a-qa]
```

`zero up` shows it in the approval prompt like any capability (`+ capability k8s.read: dev (team-a, team-a-qa)`).

### Flow

1. `zero up`: zero-cp, on the host as the human, runs the equivalent of `kubectl --context dev -n team-a create token zero-agent --duration 1h --bound-object-kind Secret --bound-object-name zero-<session>` after creating that empty Secret. The human's login method does not matter, it is whatever the kubeconfig context uses (K6).
2. The token is bound to the Secret: deleting the Secret ends this session's token at once 🟡, without touching other sessions (K5). `zero stop`, `zero rm` and the orphan sweep delete it.
3. The sandbox gets a kubeconfig with server URL and CA, no credential of the human. The token is injected at the gateway like the GitLab token if the gateway can do that for the API server (KF1); else it lies in the sandbox kubeconfig.
4. Egress: only the API server host and port of the listed contexts.
5. zero-cp renews before expiry. If the human's login has expired, renewal fails; zero-cp does not open a browser, `zero status` says `k8s dev: login expired, run: kubectl --context dev get ns`.

### Audit chain (K8)

| Step | Where | Entry |
|---|---|---|
| Human logs in | identity provider / CloudTrail | as today |
| Token issued | Kubernetes audit log | user = human, `create serviceaccounts/token zero-agent`, bound Secret `zero-<session>` |
| Agent reads | Kubernetes audit log | user = `system:serviceaccount:team-a:zero-agent`, token ID in `authentication.kubernetes.io/credential-id` 🟡 (Kubernetes ≥ 1.30) |
| Link | zero-cp decision log | session, human, context, namespace, Secret name, token ID, expiry |

The Secret name carries the session, so the first two rows are joined inside the cluster log alone.

### Guardrails for the human's own access (`zero doctor`)

zero cannot fix M1/M2, but it reads the kubeconfig context and says so:

- warn: context uses a client certificate or a static token (M1, M2);
- warn: human is cluster-admin there (`kubectl auth can-i '*' '*'`), with the hint that the agent still only gets `zero-read`;
- error: `k8s.read` asked for a context where `zero-token` is missing, with the manifest command.

With M1 `system:masters` option C still works and the agent is still read-only; the weak point stays the human's key file on the laptop, outside the sandbox.

## 6. Flaws and open points

| # | Sev | Flaw | Proposal |
|---|---|---|---|
| KF1 | M | **Token placement.** Injection at the gateway needs TLS interception towards an API server with a private CA; untested with OpenShell. | Test. Fallback: token in the sandbox, 10 min lifetime, read-only without Secrets, egress only to that API server. Decide by ADR. |
| KF2 | M | **Long streams.** `kubectl logs -f` and `watch` through the gateway are untested. | Test; else document "no follow". |
| KF3 | M | **Cluster not reachable from the sandbox** (VPN on the host, private endpoint, kind on localhost). | Test per case; else zero-cp as forward proxy for exactly that host. |
| KF4 | M | **Read leaks secrets anyway**: logs, ConfigMaps, env in pod specs, Helm release data. They reach the model provider. | ConfigMaps optional in the Role; prod excluded by default (KF6); output sanitising is a later layer. |
| KF5 | L | **Minimum token lifetime is 10 minutes** and the API server may cap the maximum. | Accept; Secret deletion is the real revocation. |
| KF6 | M | **Prod.** Same mechanism, higher damage of a leak. | Off unless the policy names the context and the user confirms the context name at `zero up`. |
| KF7 | L | **Admin must apply a manifest.** Without it no agent access. | Intended: that is the cluster owner's consent. |
| KF8 | L | Old clusters (< 1.24 🟡) lack bound object tokens for Secrets or the token ID in the audit log. | State minimum version after the test. |
| KF9 | M | Aggregated or wildcard roles added later can widen `zero-agent` unnoticed. | `zero doctor` checks `kubectl auth can-i --as system:serviceaccount:<ns>:zero-agent` for secrets, exec, create and fails if any is allowed. |

## 7. Proof plan (needs a cluster, not run yet)

1. kind cluster with audit logging on; apply the manifest for one namespace.
2. As a non-admin user (client certificate without `system:masters`, then OIDC if available): create the bound token.
3. With the token: `get pods`, `logs` work; `get secrets`, `exec`, `port-forward`, `delete pod`, `pods/proxy` are denied.
4. Delete the Secret: the token is rejected at once.
5. Audit log shows both entries of §5 with the same Secret name and token ID.
6. From a sandbox: API server reachable (KF3), token injected at the gateway or not (KF1), `logs -f` (KF2).

## 8. Questions for review

1. Option C as the base, B and D only later?
2. If gateway injection does not work (KF1): is a 10-minute read-only token inside the sandbox acceptable?
3. Prod read (KF6): off by default with explicit confirmation, or never?
4. ConfigMaps in `zero-read` by default (KF4)?
