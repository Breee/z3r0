# Gitops manifesto: the practices zero assumes

Status: draft, 2026-11-19, #125. Rules are assumptions, not all are tested; the last column says what zero itself does about a rule.
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

### 3.0 What is fixed and what depends on the organisation

The principles in section 2 are fixed: without them the claim in section 1 does not hold. How an organisation
reaches them is its own choice. The rules below state the outcome; where a rule names a tool or a layout, that is an
example.

| Fixed outcome | Depends on the organisation |
|---|---|
| No change reaches a protected branch without a human other than the agent (R1, R3). | Number of approvers, code owners or a second maintainer, who may merge. |
| The agent cannot change control files unseen (R4). | Where control files live: same repo with code owners, separate repo, or a platform repo (R14, R15). |
| Stage and control boundaries are clear (R14). | One repo per service or monorepo, directories or branches, naming. |
| Agent code in a pipeline reaches no secret and no cluster (R8, R9). | Runner setup, protected variables or an external secret store, OIDC or not (R10). |
| A stage reaches only its own data and credentials (R20 to R25). | One cluster per stage or shared cluster with admission policies, which admission tool, which secret store. |
| A change to live is a separate human decision (R17). | Promotion tool (Kargo, pipeline, second merge request), sync windows. |
| Sources of charts and images are limited (R19). | Central catalog, allow list per project, or signature checks. |

Owner: **team** (owns the service repo), **platform** (owns clusters and shared repos), **zero**.
zero's part: **enforces** = zero does it itself; **checks** = `zero doctor` reports when the rule is not met; **could check** = possible, not built; **none** = outside zero's view, the owner has to make sure.

### 3.1 Forge and repository

| # | Rule | Risk without it | Owner | zero's part |
|---|---|---|---|---|
| R1 | The default branch is protected: no direct push, no force push. | Agent or stolen token changes main without review ([P25](attack-paths.md#p25)). | team | checks (`zero doctor`) |
| R2 | Merge needs a green pipeline. | Broken or unrendered change reaches the cluster. | team | checks (`zero doctor`) |
| R3 | Merge needs approval by a human who is not the author. The agent's human is the author. | One tired human approves their own agent's change ([P16](attack-paths.md#p16), [P30](attack-paths.md#p30)). | team | could check, not built |
| R4 | Control files have code owners from the platform team: Argo CD `Application` and `AppProject`, Kargo files, `.gitlab-ci.yml`, `CODEOWNERS`, `.zero/`. | Agent changes where a stage deploys to, or what the pipeline runs ([P14](attack-paths.md#p14), [P16](attack-paths.md#p16), [P37](attack-paths.md#p37)). | team + platform | could check, not built |
| R5 | The agent pushes only to its session branch `zero/<name>`. | Agent changes other people's branches ([P25](attack-paths.md#p25)). | zero | enforces (push gate) |
| R6 | Agent instructions (`AGENTS.md`, skills, tool configs) change only by reviewed merge request. | Poisoned instructions steer every later session ([P13](attack-paths.md#p13), [P27](attack-paths.md#p27)). | team | none |
| R7 | The human reads the diff on the forge, not the agent's summary of it. | Summary hides the harmful line ([P15](attack-paths.md#p15), [P31](attack-paths.md#p31)). | team | none |

### 3.2 Pipelines

| # | Rule | Risk without it | Owner | zero's part |
|---|---|---|---|---|
| R8 | Every CI variable that is a secret is protected (and masked). Pipelines on unprotected branches get none. | Agent's branch pipeline prints the secret ([P14](attack-paths.md#p14)). | team | checks (`zero doctor`) |
| R9 | Runners for unprotected branches are separate from runners that deploy, and have no cluster or cloud access. | Agent code on the runner reaches the cluster ([P14](attack-paths.md#p14)). | platform | none |
| R10 | Pipelines authenticate with short-lived tokens (OIDC / job token), not stored keys. | A leaked variable is valid for months. | platform | none |
| R11 | The pipeline of a merge request renders every stage and runs the cluster's admission policies against the result. | Reviewer sees `values.yaml`, not what it becomes ([P37](attack-paths.md#p37)). | team | none |
| R12 | Agent pushes start no pipeline unless the project allows it (`ci.run`). | Any push is code execution outside the sandbox ([P14](attack-paths.md#p14)). | zero | enforces |
| R13 | Dependencies are pinned (lock files, image digests, chart versions). | A changed upstream runs in CI or on the host ([P17](attack-paths.md#p17), [P28](attack-paths.md#p28)). | team | none |

### 3.3 Gitops repository

| # | Rule | Risk without it | Owner | zero's part |
|---|---|---|---|---|
| R14 | The repo layout makes two things clear by path or by repo: which stage a file belongs to, and which files are control files (R4). The layout itself is free: one repo per service or a monorepo, stages as directories or as branches. With stage branches, every stage branch is protected like the default branch (R1). | Code owners, `write` paths, and the reviewer cannot tell a harmless change from one that moves a stage ([P37](attack-paths.md#p37)). In a monorepo the agent token covers every service in it (R30). | team | none |
| R15 | Platform configuration (namespaces, admission policies, storage classes, secret stores, AppProjects of other teams) is in a separate repo that is never an agent workspace. | Agent proposes a change to its own guards. | platform | could check, not built |
| R16 | No secrets in git. Stage manifests hold references (External Secrets), the store holds values. | Every clone, every sandbox, and the model provider hold the secret ([P34](attack-paths.md#p34), [P36](attack-paths.md#p36)). | team | none |
| R17 | Promotion between stages is a commit by the promotion tool (Kargo). Promotion to live is manual. | Agent's merge request to dev reaches live without a second look. | platform | none |
| R18 | Argo CD prunes and self-heals, and shows drift. Manual changes in the cluster do not survive. | Cluster state differs from git; the agent debugs a system that git does not describe. | platform | none |
| R19 | Charts and images come from catalogs the platform controls; a service repo cannot name an arbitrary source. | Merged change pulls an attacker's chart or image ([P17](attack-paths.md#p17)). | platform | none |

### 3.4 Cluster

Manifests for R20 to R26: [scenarios/kubernetes-hardening.md](scenarios/kubernetes-hardening.md).

| # | Rule | Risk without it | Owner | zero's part |
|---|---|---|---|---|
| R20 | One Argo CD `AppProject` per stage: one destination namespace, listed source repos, no cluster-scoped kinds. | Merged change deploys into another stage's namespace or creates cluster-wide objects ([P37](attack-paths.md#p37)). | platform | could check, not built |
| R21 | Namespaces enforce Pod Security `restricted`. | Merged pod mounts host paths or runs privileged. | platform | could check, not built |
| R22 | Admission policy binds storage to the stage: no `volumeName`, no cross-namespace data source, storage class of the stage, no inline CSI volumes. | qa mounts the live volume ([P37](attack-paths.md#p37)). | platform | could check, not built |
| R23 | NetworkPolicy: default deny egress, allow list per namespace. | qa connects to the live database. | platform | could check, not built |
| R24 | One secret store role per namespace; a stage can read only its own paths. | qa references the live credential ([P37](attack-paths.md#p37)). | platform | none |
| R25 | Each stage has its own credentials for databases, buckets, and queues. live is on its own cluster or at least its own nodes and storage. | A reachable host plus a shared password equals access. | platform | none |
| R26 | Humans reach clusters with short-lived logins (OIDC, cloud SSO). No static admin kubeconfig on laptops. | A stolen laptop file is cluster admin. | platform | could check, not built |
| R27 | The agent has its own ServiceAccount per session, bound to read Roles the platform installed. No Secrets, no exec, no writes. | Agent acts as the human, or changes the cluster past git. | zero + platform | designed, proven on kind (#123), not built |
| R28 | The API server writes an audit log that the platform keeps. | No answer to "what did the agent read". | platform | none |
| R29 | Logs and ConfigMaps hold no secrets. | Agent read sends them to the model provider ([P29](attack-paths.md#p29)). | team | none |

### 3.5 Workstation and session

| # | Rule | Risk without it | Owner | zero's part |
|---|---|---|---|---|
| R30 | A workspace holds only the repos and input folders the task needs. | One token covers projects the task never touches. | human | shows (`zero ls`, `zero status`) |
| R31 | Policy changes (`.zero/policy.yaml`) are read before approval. | Agent widens its own rights ([P16](attack-paths.md#p16)). | human | enforces (prompt) |
| R32 | Agent changes reach the host checkout only through the forge, after review. Hooks and tool configs from an agent branch are not run on the host. | Agent code runs as the human ([P13](attack-paths.md#p13)). | human | none |
| R33 | Input folders hold documents, not credentials. | Secret under an unknown file name is copied ([P34](attack-paths.md#p34)). | human | enforces (pattern list) |

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
