# Platform engineer — basic tasks with an agent

Status: **draft for review** (#94). Input for the target architecture (`docs/architecture/28-target-architecture.md`): the tasks decide which layers and capabilities zero needs. Nothing beyond the GitLab part is built or tested; statements about Argo CD, Kargo and Grafana come from their documentation, not from a test here.

Related: `docs/architecture/12-reference-scenario-a-kubernetes-operator.md` (older, wider scenario), `docs/scenarios/identity-broker-usecases.md` (UC2: read a branch environment).

## 1. Setting

Priya is a platform engineer. Her team runs three clusters (dev, qa, prod) for 9 product teams.

| System | Role | How Priya uses it today |
|---|---|---|
| GitLab | forge: `platform/gitops` (cluster and app manifests, Helm values), `platform/charts`, app repos | git, `glab`, MRs, pipelines |
| Kubernetes | runtime | `kubectl` with her own OIDC login |
| Argo CD | applies `platform/gitops` to the clusters | UI, `argocd app get/diff/sync` |
| Kargo | promotes a build (Freight) dev → qa → prod by writing to `platform/gitops` | UI, `kargo promote` |
| Grafana | dashboards, alerts; Prometheus and Loki behind it | UI, Explore |

Her zero workspace: `zero ctx new platform ~/src/gitops ~/src/charts ~/docs/runbooks` — two repos and one read-only input folder (runbooks).

## 2. Rule that follows from GitOps

**The agent reads the systems and writes to git.** Every change to a cluster is a commit in `platform/gitops`; Argo CD and Kargo carry it out after a human merged it. So the agent needs:

- write: only the session branch in GitLab (built: L3–L5a);
- read: Kubernetes, Argo CD, Kargo, Grafana (not built);
- no direct write to any of the four runtime systems by default.

Decision (2026-11-18): git is the authority, an agent changes nothing in the cluster (no restart, scale or patch). The few direct writes outside the cluster (sync, promote, silence) are listed in section 4 as separate capabilities, off by default.

## 3. Tasks

Status: ✅ works with zero today · 🟡 git part works, the agent lacks read access to a system · ❌ not possible today.

### GitLab only

| # | Task | Agent needs | Status |
|---|---|---|---|
| T1 | Bump an image tag or chart version in `gitops`, open the MR | `git.push`, `gitlab.mr.create` | ✅ |
| T2 | Onboard a team: namespace, quota, network policy, RBAC, Argo CD Application from a template | same; runbook from the input folder | ✅ |
| T3 | Change in two repos at once: new value in `charts`, use it in `gitops` | multi-folder workspace, one token for both projects | ✅ (#76) |
| T4 | Pipeline of the MR is red: read the job log, fix, push again | `gitlab.pipeline.read` | ✅ a human starts the pipeline unless `ci.run` is approved (ADR 0035) |
| T5 | Review someone else's MR in `gitops`: explain the diff, comment | `gitlab.mr.read`; commenting needs a capability that does not exist (`gitlab.mr.comment`) | 🟡 read only |
| T6 | Render the change locally (`helm template`, `kustomize build`, `kubeconform`) before pushing | tools in the image; chart registries in `network.allow` | ✅ if the image has the tools; not checked |

### Kubernetes

| # | Task | Agent needs | Status |
|---|---|---|---|
| T7 | "Why is `payments/ledger` crash-looping in qa?" — pods, events, logs, rollout history | read on pods, events, workloads, logs in named namespaces. No Secrets, no exec | ❌ |
| T8 | Compare what runs with what git says (drift), then fix git | read on the resources named in the manifests | ❌ |
| T9 | Capacity question: requests/limits vs. real usage per namespace | read on workloads, metrics API | ❌ |
| T10 | Cluster upgrade preparation: find deprecated API versions in use | read on all resources, cluster-wide, without Secrets | ❌ |

### Argo CD and Kargo

| # | Task | Agent needs | Status |
|---|---|---|---|
| T11 | "Why is the app OutOfSync / Degraded?" — app status, sync result, diff, resource tree | Argo CD read on named projects/apps | ❌ |
| T12 | After the MR is merged: watch the sync until healthy, report | same, plus waiting | ❌ |
| T13 | "What runs where?" — which Freight is in dev, qa, prod; what is waiting for promotion | Kargo read on a project | ❌ |
| T14 | Promotion failed: read the promotion steps and the verification result, fix the cause in git | Kargo read; write through GitLab | ❌ |

### Grafana

| # | Task | Agent needs | Status |
|---|---|---|---|
| T15 | Alert fired: which rule, since when, query the metric and the logs around it | Grafana read: alert rules, alert state, datasource queries (Prometheus, Loki) | ❌ |
| T16 | Did the change help? Compare a metric before and after the deploy | datasource query | ❌ |
| T17 | Add or change a dashboard or alert rule | none in Grafana if dashboards and rules are files in `gitops` (provisioning); otherwise Grafana write | ✅ as code / ❌ in the UI store |

### One task end to end (T15 → T7 → T11 → T1 → T12)

1. Alert "ledger 5xx > 2 %" fires. Priya: "look at it".
2. Agent queries Grafana: error rate rose at 14:02. Loki: `connection refused` to `ledger-db`.
3. Agent reads Kubernetes: pods fine, a NetworkPolicy changed at 14:01.
4. Agent reads Argo CD: app `ledger` synced revision `abc123` at 14:01.
5. Agent finds the commit in `gitops`, writes the fix on `zero/platform`, opens the MR.
6. Priya reviews, starts the pipeline, merges. Argo CD syncs.
7. Agent watches the sync and the metric, reports "error rate back under 0.1 %".

Steps 2–4 and 7 are impossible today. Without them the agent only does step 5, and Priya copies the findings in by hand — which is how unreviewed text (P26) gets into the session.

## 4. Capabilities the tasks need

Read (enough for T1–T17 except the UI half of T17):

| Capability | Meaning | Scope | Open point |
|---|---|---|---|
| `k8s.resource.read` | get/list/watch, logs. Never Secrets, exec, port-forward | cluster + namespaces | T10 needs cluster-wide read; ConfigMaps often hold secrets too |
| `argocd.read` | applications, sync status, diff, resource tree, events | Argo CD project | the diff and live manifests can show Secret data unless Argo CD masks it; pod logs through Argo CD are a second path to `k8s.resource.read` |
| `kargo.read` | stages, freight, promotions, verifications | Kargo project | Kargo permissions are Kubernetes RBAC on its own resources; a second API to cover |
| `grafana.read` | dashboards, alert rules and state, datasource queries | folder + datasources | a query can return anything the datasource holds: logs contain tokens and personal data. Loki/Prometheus tenant limits are the real scope, not Grafana's |
| `gitlab.mr.comment` | comment on an MR | project | spam only, reversible |

Write, off by default, each one a decision of its own:

| Capability | Task | Why not through git |
|---|---|---|
| `argocd.sync` | sync or refresh one app | auto-sync is off for prod in many setups |
| `kargo.promote` | promote Freight to a stage, never to prod | promotion is the release decision |
| `grafana.silence` | silence one alert for a limited time | incident handling |

Everything else (apply, delete, exec, Secret read, RBAC) stays with the human or the pipeline.

## 5. What this asks of the target architecture

| # | Gap | Layer | Note |
|---|---|---|---|
| G1 | Credentials for systems other than the forge. Today the broker mints one GitLab token; nothing for Kubernetes, Argo CD, Kargo, Grafana | L3 | each system needs: who mints, how short, how scoped. Kubernetes has TokenRequest (10 min minimum). Argo CD, Kargo and Grafana tokens are longer-lived and coarser |
| G2 | "Sandbox ≤ human" for these systems: the agent's read access must not exceed Priya's | L3, L7 | Kubernetes, Argo CD and Kargo can all log in with the same OIDC provider; Grafana too. Whether zero can derive a narrower credential from Priya's login, or needs a service account per team, is open (same question as #91 for GitLab) |
| G3 | Read is not harmless: logs, Argo CD diffs and metrics labels carry secrets, and whatever the agent reads goes to the model provider | L1, L2 | no layer filters content today ("model traffic content" in §28.3 holes). Needs a decision: accept for dev/qa, deny for prod, or filter |
| G4 | Read is an injection path: a log line or an annotation written by a workload reaches the agent as text | P26 | same as issue comments (#110); a platform engineer's agent reads other teams' output all day |
| G5 | Network reach: cluster APIs, Argo CD and Grafana are often only reachable through VPN or inside the cluster | L1, deployments | laptop sandbox may not reach them (F25 in the identity use cases); a sandbox in the cluster does |
| G6 | Method-level rules per system: read-only must hold at the gateway too, not only in the token | L5 | Kubernetes `watch` and `logs -f` are long streams, Grafana queries are POST, Kargo and Argo CD also speak gRPC. Whether OpenShell's presets can express "read only" for these is untested |
| G7 | Prod vs. dev/qa: the same capability means different risk per cluster | policy | policy needs a scope per environment, and the widening prompt has to show it |
| G8 | Tools in the image: `kubectl`, `argocd`, `kargo`, `helm`, `kustomize` | image | or MCP servers instead of CLIs; decides where G6 is enforced |
| G9 | Evidence: which human's session read what in which cluster | L8 | Kubernetes audit shows only the service account unless the token carries the session |

Smallest useful step: `k8s.resource.read` for dev/qa namespaces with a TokenRequest credential (G1 for one system, T7–T9). It was already sketched as UC2 in the identity use cases. Argo CD and Grafana read follow the same pattern; the write capabilities come last. How the credential works for every way people log in to clusters: [kubernetes-access.md](kubernetes-access.md).

## 6. Questions for review

1. Is the rule in section 2 (agent reads systems, writes only to git) the right default for a platform engineer, or is direct write in dev expected?
2. Prod: may the agent read prod logs and metrics at all (G3)?
3. Which system first after GitLab: Kubernetes, or Grafana because incidents start there?
4. CLIs in the sandbox or MCP servers outside it (G8)?
5. Tasks missing from section 3?
