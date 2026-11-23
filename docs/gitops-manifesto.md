# Gitops manifesto: the practices zero assumes

Status: draft 2, 2026-11-19, #125. Rules come from the sources in section 9; none is tested by zero unless the
"zero" column says so.
Related: [scenarios/kubernetes-hardening.md](scenarios/kubernetes-hardening.md) (example manifests),
[scenarios/kubernetes-access.md](scenarios/kubernetes-access.md), [scenarios/platform-engineer-usecases.md](scenarios/platform-engineer-usecases.md),
[attack-paths.md](attack-paths.md).

## 1. The claim

With zero, the only thing an agent can produce is a branch and a merge request.

zero enforces that inside the sandbox. It does not decide what happens to the merge request afterwards: who
reviews it, what the pipeline runs, what the delivery system applies. That part belongs to the team and the
platform. This file lists what must hold there so that a bad merge request stays a bad merge request.

A corrupted agent (prompt injection, model error) has two ways left:

1. **Before the merge:** the push itself does damage, for example a pipeline on the agent's branch runs with secrets.
2. **After the merge:** a human merges a change that looks harmless, and the delivery system executes it with full rights.

## 2. The path to production and its five gates

Every change takes the same path. Each gate stops a different kind of bad change. The rules in section 4 are
grouped by gate.

```
agent ──push──▶ branch ──MR──▶ main ──promote──▶ stage config ──reconcile──▶ cluster
        │              │               │                     │                  │
      gate 2         gate 1          gate 3                gate 4             gate 5
      pipeline       review          repository            delivery           cluster
```

| Gate | Question it answers | Stops |
|---|---|---|
| 1 Review | Did a human who is allowed to decide see this change? | unreviewed change on the default branch |
| 2 Pipeline | What can code on an unreviewed branch reach? | damage before the merge |
| 3 Repository | Which files decide where and with which rights things run? | a change to the controls hidden in a normal change |
| 4 Delivery | What may the delivery system apply for this stage? | a merged change that leaves its stage |
| 5 Cluster | What may a workload of this stage reach? | a deployed workload that reaches other stages |

Gates 1 and 2 are needed for the claim in section 1. Gates 3 to 5 limit the damage when a bad change is merged
anyway. Reviews fail (P30, P31), so they are not optional.

## 3. How to read the rules

- **Level** `base`: without this rule the claim in section 1 is false. `depth`: limits what a merged bad change can do.
- **Owner** `team`: maintainers of the repository. `platform`: who runs forge, runners, delivery system and clusters.
- **zero** `checks`: `zero doctor` reports a violation today. `could check`: possible through an API zero already
  uses, not built. `-`: zero cannot see it.
- **Risk** links to [attack-paths.md](attack-paths.md). No link means the risk is described in the source only.
- Settings are named for GitLab, Argo CD, Kargo and External Secrets, because zero is tested with them. The rule is
  the first sentence; the setting is one way to meet it.

What is fixed and what the organisation decides:

| Fixed | The organisation decides |
|---|---|
| a human other than the agent decides about every merge | how many approvers, which roles |
| unreviewed code never runs with secrets | which runners, which secret store |
| control files have named owners | repository layout: one repo or many, directories or branches per stage |
| a stage reaches only its own data, secrets and cluster rights | one cluster or several, which policy engine |
| live changes by a recorded promotion | which promotion tool, automatic or manual below live |

## 4. Rules

One rule is one condition that can be checked by itself. The "Check" column names the setting or the place to look.
`P` is the attack path the rule closes or narrows.

### Gate 1: Review

| # | Rule | Check (GitLab) | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| F1 | Nobody pushes to the default branch. | Protected branch: "Allowed to push and merge" = No one | change reaches main without a merge request ([P25](attack-paths.md#p25)) | base | team | checks |
| F2 | Nobody force-pushes to the default branch. | Protected branch: "Allowed to force push" off | history of main is rewritten ([P25](attack-paths.md#p25)) | base | team | checks |
| F3 | A merge needs a successful pipeline. | "Pipelines must succeed" on | broken or unchecked change is merged | depth | team | checks |
| F4 | A merge needs at least one approval. | approval rule with 1 or more required approvals (Premium) | change is merged unread ([P16](attack-paths.md#p16), [P30](attack-paths.md#p30)) | base | team | could check |
| F5 | The creator of a merge request cannot approve it. | "Prevent approval by merge request creator" on (Premium) | the person who ran the agent waves its change through ([P16](attack-paths.md#p16), [P30](attack-paths.md#p30)) | base | team | could check |
| F6 | Who added a commit cannot approve. | "Prevent approvals by users who add commits" on (Premium) | co-author approves the own work | base | team | could check |
| F7 | New commits remove the approvals. | "Remove all approvals when commits are added" on (Premium) | agent adds a commit after the approval | base | team | could check |
| F8 | A merge request cannot change its own approval rules. | "Prevent editing approval rules in merge requests" on (Premium) | author removes the required approval | base | team | could check |
| F9 | Control files (section 5) change only with approval of their owners. | `CODEOWNERS` lists them; "Require approval from code owners" on the protected branch (Premium) | change to pipeline or delivery rules hidden in a normal change ([P14](attack-paths.md#p14), [P16](attack-paths.md#p16), [P37](attack-paths.md#p37)) | base | platform | could check |
| F10 | Files that instruct the agent (`AGENTS.md`, skills, `.zero/`) are control files. | `CODEOWNERS` has an entry for each | agent rewrites its own rules for the next session ([P13](attack-paths.md#p13), [P27](attack-paths.md#p27)) | base | team | could check |
| F11 | The reviewer reads the diff on the forge, not the agent's summary. | habit; no setting | summary and diff differ ([P15](attack-paths.md#p15), [P31](attack-paths.md#p31)) | base | team | - |
| F12 | No automation merges or approves a merge request from an agent branch. | no auto-merge bot; no approval by a bot or pipeline token | review is skipped (OWASP CICD-SEC-1) | base | team | - |

Two limits to know:

- **GitLab Free has no required approvals and no code owners.** F4 to F10 then rest on the one person who merges.
  Section 7 says what is left.
- **The agent works with a token of its human.** The merge request is therefore created by that human, and F5 makes
  a second person necessary. That is the intended effect. An own identity for the agent is compared in #91.

### Gate 2: Pipeline

| # | Rule | Check (GitLab) | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| C1 | A pipeline on an unprotected branch gets no secret variable. | every secret CI/CD variable is "Protected" | agent branch prints or sends the secrets ([P14](attack-paths.md#p14)) | base | team | checks |
| C2 | Secret variables do not show in job logs or settings. | every secret CI/CD variable is "Masked and hidden" | secret is read from a job log | depth | team | could check |
| C3 | Jobs do not run privileged. | runner config: `privileged = false` | job code becomes root on the runner host ([P14](attack-paths.md#p14)) | base | platform | - |
| C4 | Jobs do not get the host's container socket. | runner config: no `/var/run/docker.sock` volume; images built with a rootless builder | job code starts containers on the host and leaves its own ([P14](attack-paths.md#p14)) | base | platform | - |
| C5 | Every job starts in a fresh environment. | executor creates a new container or VM per job; no shell executor on a shared host | job reads tokens and checkouts of earlier jobs ([P14](attack-paths.md#p14)) | base | platform | - |
| C6 | Protected and unprotected branches do not share a cache. | "Use separate caches for protected branches" on | agent branch poisons the cache a deploy job uses later ([P14](attack-paths.md#p14)) | base | team | could check |
| C7 | Deploy jobs run on runners that take no unreviewed code. | deploy runners are "Protected" | agent branch runs on the runner that deploys ([P14](attack-paths.md#p14), OWASP CICD-SEC-4) | base | platform | - |
| C8 | Runners for unreviewed code have no network route to clusters or admin networks. | own network segment; test from a job: API servers and admin hosts not reachable | job code uses the runner's network position instead of a secret ([P14](attack-paths.md#p14)) | base | platform | - |
| C9 | Runners for unreviewed code have no identity with rights. | no cloud instance role, no node ServiceAccount with rights | job code uses the runner's identity instead of a secret ([P14](attack-paths.md#p14)) | base | platform | - |
| C10 | Pipelines log in to cloud and secret stores with short-lived tokens, not stored credentials. | `id_tokens` with an own `aud` per service; no long-lived deploy credential in variables | stolen deploy credential works for months | depth | platform | - |
| C11 | Only protected refs get such a login. | the service's trust rule checks the claim `ref_protected` | agent branch gets the same login as main | depth | platform | - |
| C12 | The job token of a project reaches only that project. | job token allowlist holds only what is needed | pipeline code reads or writes other projects | depth | team | could check |
| C13 | Who starts a pipeline cannot override its variables. | "Minimum role to use pipeline variables" = no one allowed | caller redirects a job, e.g. the deploy target | depth | team | could check |
| C14 | Job images are pinned. | `image:` by digest, no variable in the image reference | changed image runs in the pipeline ([P17](attack-paths.md#p17), [P28](attack-paths.md#p28)) | depth | team | - |
| C15 | Included pipeline files are pinned. | `include:` by commit SHA, protected tag or component version; no remote include without one | changed include runs in the pipeline ([P17](attack-paths.md#p17), [P28](attack-paths.md#p28)) | depth | team | - |
| C16 | What a job downloads is pinned. | lock files used (`npm ci`, `--require-hashes`, `go mod verify`); checksums for downloaded tools | changed dependency runs in the pipeline ([P17](attack-paths.md#p17), [P28](attack-paths.md#p28)) | depth | team | - |
| C17 | The merge request pipeline renders every stage. | job with `helm template` / `kustomize build` per stage; result visible in the merge request | reviewer sees a values change, not what it deploys ([P37](attack-paths.md#p37)) | depth | team | - |
| C18 | The merge request pipeline runs the cluster's admission policies on the rendered result. | job with the policy CLI (kyverno, `kubectl --dry-run=server`) | a change the cluster would reject, or should reject, is found only after the merge ([P37](attack-paths.md#p37)) | depth | team | - |

### Gate 3: Repository

| # | Rule | Check | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| G1 | Deployment configuration lives in its own repository, not next to the application source. | config repo per service or per team | every developer of the source can change what is deployed; pipeline loops | depth | team | - |
| G2 | The path of a file says which stage it belongs to. | by directory or by repository; the layout itself is free | stage limits (D1, D2) cannot be written down ([P37](attack-paths.md#p37)) | base | team | - |
| G3 | The path of a file says whether it is a control file. | control files match a few path patterns | owners (F9) cannot be written down ([P37](attack-paths.md#p37)) | base | team | - |
| G4 | Platform configuration (namespaces, admission policies, secret stores, projects of the delivery system) is in a repository that is never an agent workspace. | separate repo, platform team only | agent proposes changes to the controls that hold it | base | platform | could check |
| G5 | No secret is in git. The repository holds references only. | External Secrets or similar | secret reaches the sandbox and the model provider ([P34](attack-paths.md#p34), [P36](attack-paths.md#p36)) | base | team | - |
| G6 | A push that contains a secret is rejected. | secret push protection or a server-side hook | G5 holds only until the first mistake | depth | platform | - |
| G7 | Every remote reference is pinned. | chart version, kustomize base with `?ref=<tag or SHA>`, image digest | the same commit deploys something else tomorrow ([P17](attack-paths.md#p17)) | depth | team | - |
| G8 | Charts and images come from a catalog the platform controls. | registry and chart repository of the platform; no other source in the manifests | a remote base or dependency pulls from a repo the project does not allow ([P17](attack-paths.md#p17)) | depth | platform | - |
| G9 | A stage changes by promotion: a commit made by the promotion tool. | Kargo stage per environment | change reaches a stage without having run in the one below ([P37](attack-paths.md#p37)) | depth | platform | - |
| G10 | Promotion to live is a human decision. | Kargo: no auto-promotion for the live stage | change reaches live unseen ([P37](attack-paths.md#p37)) | depth | platform | - |

### Gate 4: Delivery

| # | Rule | Check (Argo CD, Kargo) | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| D1 | Each stage has its own project that fixes the source repositories. | `AppProject.spec.sourceRepos` without `*` | stage deploys from a repository nobody reviewed ([P37](attack-paths.md#p37)) | depth | platform | could check |
| D2 | The project fixes target cluster and namespaces. | `AppProject.spec.destinations` without `*` | merged change deploys into another stage ([P37](attack-paths.md#p37)) | depth | platform | could check |
| D3 | The project allows no cluster-wide kinds. | `clusterResourceWhitelist` empty | merged change creates ClusterRoles, namespaces, PersistentVolumes ([P37](attack-paths.md#p37)) | depth | platform | could check |
| D4 | The default project allows nothing. | `default` project: no sources, no destinations | an Application without project gets everything | depth | platform | could check |
| D5 | What is removed from git is removed from the cluster. | automated sync with `prune` | git no longer says what runs | depth | platform | - |
| D6 | A manual change in the cluster is reverted. | automated sync with `selfHeal` | manual change stays unnoticed | depth | platform | - |
| D7 | The delivery system writes only to the namespaces it manages. | Argo CD cluster role limited; not cluster-admin | write access to a trusted repository becomes cluster-admin | depth | platform | - |
| D8 | Render tools and plugins that are not used are off. | Argo CD config management tools and plugins disabled | a chart or plugin reads files of other repositories during rendering | depth | platform | - |
| D9 | The promotion tool reads Secrets only in its project namespaces. | Kargo controller default kept; no cluster-wide Secret access | promotion step reads credentials of other projects | depth | platform | - |
| D10 | Humans reach the delivery system by single sign-on only. | OIDC; local admin accounts disabled | shared admin password; action without a name | depth | platform | - |
| D11 | Who synced or promoted is recorded and kept. | Argo CD events and Kargo promotions shipped to the log store | sync or promotion cannot be traced | depth | platform | - |

### Gate 5: Cluster

| # | Rule | Check (Kubernetes) | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| K1 | One namespace per service and stage. | `shop-dev`, `shop-qa`, `shop-live` | who can create a workload in a namespace reaches every Secret and ServiceAccount in it | depth | platform | - |
| K2 | Workloads run without host access. | namespace label `pod-security.kubernetes.io/enforce=restricted` | pod mounts host paths, runs privileged | depth | platform | could check |
| K3 | A claim cannot pick an existing volume or a source in another namespace. | admission policy on PVCs: no `volumeName`, no cross-namespace `dataSourceRef` | qa workload mounts the live volume ([P37](attack-paths.md#p37)) | depth | platform | could check |
| K4 | A namespace can use only the storage class of its stage. | admission policy: `storageClassName` equals the namespace's stage label | qa workload gets storage of live ([P37](attack-paths.md#p37)) | depth | platform | could check |
| K5 | Only the platform creates PersistentVolumes. | no Role or project outside the platform may create `persistentvolumes` | hand-made volume points at live data | depth | platform | - |
| K6 | A namespace reaches only the network targets it needs. | `NetworkPolicy` default deny for egress plus allow list | qa workload talks to the live database ([P37](attack-paths.md#p37)) | depth | platform | could check |
| K7 | A namespace reads only the secrets of its stage. | `SecretStore` per namespace with a role for that stage | qa workload fetches the live credential ([P37](attack-paths.md#p37)) | depth | platform | - |
| K8 | No cluster-wide secret store is open to every namespace. | every `ClusterSecretStore` has namespace conditions | K7 is bypassed by naming the cluster-wide store ([P37](attack-paths.md#p37)) | depth | platform | - |
| K9 | Each stage has its own credentials for everything outside the cluster. | separate database users, buckets, cloud roles | one leaked credential opens every stage ([P37](attack-paths.md#p37)) | depth | platform | - |
| K10 | Live runs on its own cluster, or at least on its own nodes and storage. | separate cluster or node pool | K3 to K8 are the only wall between qa and live ([P37](attack-paths.md#p37)) | depth | platform | - |
| K11 | Humans log in to the cluster with short-lived credentials. | OIDC or cloud SSO; no client certificate or static token in a kubeconfig | stolen kubeconfig works for months | depth | platform | could check |
| K12 | Nobody uses cluster-admin for daily work. | namespace `RoleBinding`s, no wildcards; cluster-admin by separate login | any mistake is cluster-wide | depth | platform | - |
| K13 | The API audit log is kept outside the cluster. | audit policy + log shipping | nobody can say who did what | depth | platform | - |
| K14 | Logs hold no secrets. | review; secret scanning of logs | a read-only agent still reads secrets ([P29](attack-paths.md#p29)) | depth | team | - |
| K15 | ConfigMaps hold no secrets. | review; secret scanning of rendered manifests | a read-only agent still reads secrets ([P29](attack-paths.md#p29)) | depth | team | - |

## 5. Control files

Control files decide where and with which rights things run. An agent may read them. A change needs their owner (F9).

| File | Why it is a control file |
|---|---|
| `.gitlab-ci.yml` and everything it includes | decides what runs with the pipeline's rights |
| `CODEOWNERS` | decides who owns the other control files |
| `AGENTS.md`, skills, `.zero/policy.yaml` | instruct the agent and set its capabilities |
| Argo CD `Application`, `AppProject`, `ApplicationSet` | decide source, target cluster, namespace and allowed kinds |
| Kargo `Project`, `Warehouse`, `Stage`, promotion tasks, analysis templates | decide what is promoted and what counts as verified |
| namespace, admission policy, `NetworkPolicy`, `SecretStore` manifests | are the walls of gate 5; rule G4 puts them in the platform repository |

Not control files: values of a stage, application manifests of a stage. They are what the agent is there to change.

## 6. zero's side

What zero does so that the rules above are enough. Details in [architecture/28-target-architecture.md](architecture/28-target-architecture.md).

| # | zero does | Closes | Status |
|---|---|---|---|
| Z1 | The agent pushes only to its session branch `zero/<name>`; no delete, no other ref. | [P25](attack-paths.md#p25) | built (push gate) |
| Z2 | An agent push starts no pipeline unless the policy grants `ci.run`. | [P14](attack-paths.md#p14) | built for pushes; a merge request opened by the agent still starts one (#134) |
| Z3 | A policy change that widens rights is shown and needs the human's approval. | [P16](attack-paths.md#p16) | built |
| Z4 | Input folders are copied without files that look like credentials. | [P34](attack-paths.md#p34) | built |
| Z5 | The agent's forge token cannot merge, approve or change project settings, and covers only the workspace's projects. | - | built |
| Z6 | In a cluster the agent has its own ServiceAccount per session with read Roles the admin installed. | - | proven on kind (#123), not built |

What the human at the workstation does:

| # | The human | Risk without it |
|---|---|---|
| H1 | puts only the repositories and inputs into a workspace that the task needs (`zero ls`, `zero status` show them) | agent reads and changes more than the task needs |
| H2 | takes agent changes into the own checkout only from the forge, after review | agent code runs outside the sandbox ([P13](attack-paths.md#p13)) |
| H3 | reads the policy prompt before approving (Z3) | agent widens its own rights ([P16](attack-paths.md#p16)) |

## 7. When a rule cannot be met

| Situation | What is lost | What is left |
|---|---|---|
| GitLab Free: no required approvals, no code owners (F4 to F10) | a second person and file owners are not enforced | F1 to F3, F11; the person who merges is the only review. A `write` path list in the push gate would keep the agent out of control files; not built |
| One cluster for all stages (K10) | a second wall behind K3 to K8 | K1 to K8 enforced by admission; own nodes for live |
| Kubernetes below 1.30 (no ValidatingAdmissionPolicy) | built-in admission policies (K3, K4) | Kyverno or Gatekeeper |
| No promotion tool (G9, G10) | recorded promotion | merge request per stage with F4 to F9 on the stage paths |
| Runners shared by all branches (C7) | isolation of unreviewed code | do not grant `ci.run`; then agent pushes start no pipeline (Z2) |

## 8. What zero checks

- Today `zero doctor` checks F1, F2, F3 and C1 for the GitLab project around the current directory.
- Candidates, in this order: F9 and F10 (code owners exist, are required, cover the control files), F4 to F8
  (approval settings), C2, C6, C12, C13, D1 to D4, K2, K3, K4 and K6 (only in namespaces that a zero policy grants
  to the agent), K11, G4.
- zero does not check gates 4 and 5 today. It has no access to the delivery system or the cluster.

## 9. Sources

Read on 2026-11-19. A rule that names no attack path rests on these.

| Source | Used for |
|---|---|
| [OpenGitOps principles 1.0](https://opengitops.dev/): declarative, versioned and immutable, pulled automatically, continuously reconciled | D5, D6, G7 |
| [OWASP Top 10 CI/CD Security Risks](https://owasp.org/www-project-top-10-ci-cd-security-risks/): CICD-SEC-1 flow control, CICD-SEC-4 poisoned pipeline execution | F1, F4, F5, F12, C1, C7 |
| GitLab: [protected branches](https://docs.gitlab.com/user/project/repository/branches/protected/), [approval settings](https://docs.gitlab.com/user/project/merge_requests/approvals/settings/), [code owners](https://docs.gitlab.com/user/project/codeowners/) | F1, F2, F4 to F9 |
| GitLab: [pipeline security](https://docs.gitlab.com/ci/pipeline_security/), [job token](https://docs.gitlab.com/ci/jobs/ci_job_token/), [ID tokens](https://docs.gitlab.com/ci/secrets/id_token_authentication/) | C1, C2, C6, C10 to C16 |
| Argo CD: [best practices](https://argo-cd.readthedocs.io/en/stable/user-guide/best_practices/), [security](https://argo-cd.readthedocs.io/en/stable/operator-manual/security/), [projects](https://argo-cd.readthedocs.io/en/stable/user-guide/projects/) | G1, G7, G8, D1 to D8, D10, D11 |
| Kargo: [secure configuration](https://docs.kargo.io/operator-guide/security/secure-configuration/) | D9, D10 |
| Kubernetes: [RBAC good practices](https://kubernetes.io/docs/concepts/security/rbac-good-practices/), [good practices for Secrets](https://kubernetes.io/docs/concepts/security/secrets-good-practices/), [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) | K1, K2, K5, K11, K12 |
| External Secrets: [security best practices](https://external-secrets.io/latest/guides/security-best-practices/) | K7, K8 |

C3 to C5, C8, C9 and G6 rest on common runner and forge practice, not on a page read for this file.

Not read: NSA/CISA Kubernetes Hardening Guide, SLSA source track, Kargo and Argo CD ApplicationSet security pages.

## 10. Not in this file

- How zero confines the agent: [architecture/28-target-architecture.md](architecture/28-target-architecture.md).
- Example manifests for gates 4 and 5: [scenarios/kubernetes-hardening.md](scenarios/kubernetes-hardening.md).
- General cluster hardening that has nothing to do with stages or agents (node hardening, etcd encryption, image scanning, patch level of runners and controllers).
