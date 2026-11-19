# Gitops manifesto: the practices zero assumes

Status: draft, 2026-11-19, #125. Rules are assumptions, not all are tested; "check" says what zero verifies today.
Related: [scenarios/kubernetes-hardening.md](scenarios/kubernetes-hardening.md) (manifests for section 4),
[scenarios/kubernetes-access.md](scenarios/kubernetes-access.md), [scenarios/platform-engineer-usecases.md](scenarios/platform-engineer-usecases.md),
[attack-paths.md](attack-paths.md).

## 1. Why this file

zero limits what an agent can do from inside its sandbox. It does not review what a human merges, and it does not
guard the cluster. An agent that is corrupted (prompt injection, model error) will try the next best thing: write a
change that a human merges and the delivery system then executes with full rights.

zero is as safe as the delivery system around it. This file lists the practices under which the statement
"the worst an agent can do is open a bad merge request" is true. Where a practice is missing, the attack path in the
last column of each rule is open.

## 2. Principles

1. **Git is the authority.** Every change to a running system is a commit. Nobody, human or agent, patches a cluster.
2. **The agent reads the systems and writes to git.** Its only write is a branch and a merge request.
3. **A human merges.** The merge is the approval. The agent holds no right to merge, sync, promote, or release.
4. **Controls live outside the agent's reach.** The files and settings that decide what a change may do are not in
   the part of the repo the agent can change, or are not in the repo at all.
5. **A stage reaches only what belongs to that stage.** dev, qa, and live do not share data, credentials, or storage.
6. **The cluster enforces, the pipeline warns.** A check that runs in CI on the agent's branch can be changed by the
   agent. A check in admission or in the forge settings cannot.
7. **Credentials are short-lived, narrow, and never in git.** For humans, pipelines, and agents alike.
8. **Every action has a name.** An agent acts under its own identity, traceable to the human who started it.

## 3. Rules

Owner: **team** (owns the service repo), **platform** (owns clusters and shared repos), **zero**.
Check: **doctor** = `zero doctor` checks it today; **could** = zero could check it, not built; **no** = outside zero's view.

### 3.1 Forge and repository

| # | Rule | Without it | Owner | Check |
|---|---|---|---|---|
| R1 | The default branch is protected: no direct push, no force push. | Agent or stolen token changes main without review (P25). | team | doctor |
| R2 | Merge needs a green pipeline. | Broken or unrendered change reaches the cluster. | team | doctor |
| R3 | Merge needs approval by a human who is not the author. The agent's human is the author. | One tired human approves their own agent's change (P16, P30). | team | could |
| R4 | Control files have code owners from the platform team: Argo CD `Application` and `AppProject`, Kargo files, `.gitlab-ci.yml`, `CODEOWNERS`, `.zero/`. | Agent changes where a stage deploys to, or what the pipeline runs (P14, P16, P37). | team + platform | could |
| R5 | The agent pushes only to its session branch `zero/<name>`. | Agent changes other people's branches (P25). | zero | built (push gate) |
| R6 | Agent instructions (`AGENTS.md`, skills, tool configs) change only by reviewed merge request. | Poisoned instructions steer every later session (P13, P27). | team | no |
| R7 | The human reads the diff on the forge, not the agent's summary of it. | Summary hides the harmful line (P15, P31). | team | no |

### 3.2 Pipelines

| # | Rule | Without it | Owner | Check |
|---|---|---|---|---|
| R8 | Every CI variable that is a secret is protected (and masked). Pipelines on unprotected branches get none. | Agent's branch pipeline prints the secret (P14). | team | doctor |
| R9 | Runners for unprotected branches are separate from runners that deploy, and have no cluster or cloud access. | Agent code on the runner reaches the cluster (P14). | platform | no |
| R10 | Pipelines authenticate with short-lived tokens (OIDC / job token), not stored keys. | A leaked variable is valid for months. | platform | no |
| R11 | The pipeline of a merge request renders every stage and runs the cluster's admission policies against the result. | Reviewer sees `values.yaml`, not what it becomes (P37). | team | no |
| R12 | Agent pushes start no pipeline unless the project allows it (`ci.run`). | Any push is code execution outside the sandbox (P14). | zero | built |
| R13 | Dependencies are pinned (lock files, image digests, chart versions). | A changed upstream runs in CI or on the host (P17, P28). | team | no |

### 3.3 Gitops repository

| # | Rule | Without it | Owner | Check |
|---|---|---|---|---|
| R14 | One config repo per service. Stages are directories, not branches. | Agent workspace covers more services than the task; branch drift hides differences. | team | no |
| R15 | Platform configuration (namespaces, admission policies, storage classes, secret stores, AppProjects of other teams) is in a separate repo that is never an agent workspace. | Agent proposes a change to its own guards. | platform | could |
| R16 | No secrets in git. Stage manifests hold references (External Secrets), the store holds values. | Every clone, every sandbox, and the model provider hold the secret (P34, P36). | team | no |
| R17 | Promotion between stages is a commit by the promotion tool (Kargo). Promotion to live is manual. | Agent's merge request to dev reaches live without a second look. | platform | no |
| R18 | Argo CD prunes and self-heals, and shows drift. Manual changes in the cluster do not survive. | Cluster state differs from git; the agent debugs a system that git does not describe. | platform | no |
| R19 | Charts and images come from catalogs the platform controls; a service repo cannot name an arbitrary source. | Merged change pulls an attacker's chart or image (P17). | platform | no |

### 3.4 Cluster

Manifests for R20 to R26: [scenarios/kubernetes-hardening.md](scenarios/kubernetes-hardening.md).

| # | Rule | Without it | Owner | Check |
|---|---|---|---|---|
| R20 | One Argo CD `AppProject` per stage: one destination namespace, listed source repos, no cluster-scoped kinds. | Merged change deploys into another stage's namespace or creates cluster-wide objects (P37). | platform | could |
| R21 | Namespaces enforce Pod Security `restricted`. | Merged pod mounts host paths or runs privileged. | platform | could |
| R22 | Admission policy binds storage to the stage: no `volumeName`, no cross-namespace data source, storage class of the stage, no inline CSI volumes. | qa mounts the live volume (P37). | platform | could |
| R23 | NetworkPolicy: default deny egress, allow list per namespace. | qa connects to the live database. | platform | could |
| R24 | One secret store role per namespace; a stage can read only its own paths. | qa references the live credential (P37). | platform | no |
| R25 | Each stage has its own credentials for databases, buckets, and queues. live is on its own cluster or at least its own nodes and storage. | A reachable host plus a shared password equals access. | platform | no |
| R26 | Humans reach clusters with short-lived logins (OIDC, cloud SSO). No static admin kubeconfig on laptops. | A stolen laptop file is cluster admin. | platform | could |
| R27 | The agent has its own ServiceAccount per session, bound to read Roles the platform installed. No Secrets, no exec, no writes. | Agent acts as the human, or changes the cluster past git. | zero + platform | proven on kind (#123), not built |
| R28 | The API server writes an audit log that the platform keeps. | No answer to "what did the agent read". | platform | no |
| R29 | Logs and ConfigMaps hold no secrets. | Agent read sends them to the model provider (P29). | team | no |

### 3.5 Workstation and session

| # | Rule | Without it | Owner | Check |
|---|---|---|---|---|
| R30 | A workspace holds only the repos and input folders the task needs. | One token covers projects the task never touches. | human | `zero ls`, `zero status` |
| R31 | Policy changes (`.zero/policy.yaml`) are read before approval. | Agent widens its own rights (P16). | human | built (prompt) |
| R32 | Agent changes reach the host checkout only through the forge, after review. Hooks and tool configs from an agent branch are not run on the host. | Agent code runs as the human (P13). | human | no |
| R33 | Input folders hold documents, not credentials. | Secret under an unknown file name is copied (P34). | human | built (pattern list) |

## 4. What zero checks today

`zero doctor`, for the repository around the current directory on GitLab: R1 (protected, no direct push, no force
push), R2, R8. zero itself enforces R5, R12, R31, R33.

Candidates for `zero doctor`, in order of value: R4 (control files have code owners), R3 (approval rule),
R20 to R23 on every namespace an agent is granted (needs the Kubernetes access of R27 first), R26 (kubeconfig holds a
static credential), R15 (workspace contains a repo marked as platform repo).

## 5. When a rule cannot be met

- No code owners (GitLab Free): the push gate could refuse pushes that touch control files (`write` paths in
  `.zero/policy.yaml`, not built). It protects against the agent only, not against other authors.
- One cluster for all stages: R22 to R25 carry the whole separation. Treat them as required.
- No admission policies (Kubernetes < 1.30): use an admission controller (Kyverno, Gatekeeper) with the same rules.
- No promotion tool: R17 becomes "a second human merges the change to live".

## 6. Not in this file

- How zero confines the agent on the workstation (sandbox, egress, tokens): [architecture/28-target-architecture.md](architecture/28-target-architecture.md).
- Application security, image scanning, backup. They matter, but they do not change what an agent can do.
