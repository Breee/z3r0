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

### Gate 1: Review

| # | Rule | How to meet it (GitLab) | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| F1 | Nobody pushes to the default branch. Changes arrive by merge request only. | Protected branch: "Allowed to push and merge" = No one, "Allowed to force push" off | agent or anyone rewrites main ([P25](attack-paths.md#p25)) | base | team | checks |
| F2 | A merge needs a successful pipeline. | "Pipelines must succeed" | broken or unchecked change is merged | depth | team | checks |
| F3 | A human who did not write the change approves it. New commits remove the approval. | Approval rule with at least 1 approval; "Prevent approval by merge request creator", "Prevent approvals by users who add commits", "Remove all approvals when commits are added", "Prevent editing approval rules in merge requests" (Premium) | the person who ran the agent waves its change through ([P16](attack-paths.md#p16), [P30](attack-paths.md#p30)); agent adds a commit after approval | base | team | could check |
| F4 | Control files (section 5) change only with approval of their owners. | `CODEOWNERS` + "Require approval from code owners" on the protected branch (Premium) | change to pipeline, delivery or agent rules hidden in a normal change ([P14](attack-paths.md#p14), [P16](attack-paths.md#p16), [P37](attack-paths.md#p37)) | base | platform | could check |
| F5 | The reviewer reads the diff on the forge, not the agent's summary. | habit; no setting | summary and diff differ ([P15](attack-paths.md#p15), [P31](attack-paths.md#p31)) | base | team | - |
| F6 | Files that instruct the agent (`AGENTS.md`, skills, `.zero/`) are control files. | listed in `CODEOWNERS` | agent rewrites its own rules for the next session ([P13](attack-paths.md#p13), [P27](attack-paths.md#p27)) | base | team | - |
| F7 | No automation merges or approves a merge request from an agent branch. | no auto-merge bots, no approval by pipeline token | review is skipped (OWASP CICD-SEC-1) | base | team | - |

Two limits to know:

- **GitLab Free has no required approvals and no code owners.** F3 and F4 then rest on the one person who merges.
  Section 7 says what is left.
- **The agent works with a token of its human.** The merge request is therefore created by that human, and
  "Prevent approval by merge request creator" makes a second person necessary. That is the intended effect of F3.
  An own identity for the agent is compared in #91.

### Gate 2: Pipeline

| # | Rule | How to meet it (GitLab) | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| C1 | A pipeline on an unprotected branch gets no secrets. | every secret CI/CD variable "Protected", also "Masked and hidden"; better: secrets in a secrets manager, fetched by the job | agent branch prints or sends the secrets ([P14](attack-paths.md#p14)) | base | team | checks |
| C2 | Unreviewed code runs on runners that reach nothing else. | deploy runners "Protected"; runners for other branches separate, not privileged, no route to clusters or cloud accounts | pipeline code leaves the job ([P14](attack-paths.md#p14), OWASP CICD-SEC-4) | base | platform | - |
| C3 | Pipelines log in to cloud and secret stores with short-lived tokens bound to protected refs. | `id_tokens` with own `aud` per service; the service checks the claim `ref_protected` | stolen long-lived deploy credential; agent branch gets the same login as main | depth | platform | - |
| C4 | The job token of a project reaches only that project. | job token allowlist holds only what is needed | pipeline code reads or writes other projects | depth | team | could check |
| C5 | Who starts a pipeline cannot override its variables. | "Minimum role to use pipeline variables" = no one allowed; use `inputs` | caller redirects a job, e.g. the deploy target | depth | team | could check |
| C6 | Everything the pipeline loads is pinned. | images by digest, `include:` by commit SHA or protected tag, lock files, checksums for downloaded tools | changed dependency runs in the pipeline ([P17](attack-paths.md#p17), [P28](attack-paths.md#p28)) | depth | team | - |
| C7 | The merge request pipeline renders every stage and runs the cluster's admission policies on the result. | job with `helm template` / `kustomize build` + policy CLI (kubeconform, kyverno, `kubectl --dry-run=server`) | reviewer sees a values change, not what it deploys ([P37](attack-paths.md#p37)) | depth | team | - |

### Gate 3: Repository

| # | Rule | How to meet it | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| G1 | Deployment configuration lives in its own repository, not next to the application source. | config repo per service or per team | every developer of the source can change what is deployed; pipeline loops | depth | team | - |
| G2 | The layout shows which files belong to which stage and which files are control files. | by path or by repository; the layout itself is free | owners (F4) and stage limits (D1) cannot be written down ([P37](attack-paths.md#p37)) | base | team | - |
| G3 | Platform configuration (namespaces, admission policies, secret stores, projects of the delivery system) is in a repository that is never an agent workspace. | separate repo, platform team only | agent proposes changes to the controls that hold it | base | platform | could check |
| G4 | No secret is in git. The repository holds references only. | External Secrets or similar; secret detection on push | secret reaches the sandbox and the model provider ([P34](attack-paths.md#p34), [P36](attack-paths.md#p36)) | base | team | - |
| G5 | Every remote reference is pinned and comes from a catalog the platform controls. | chart version, kustomize base with `?ref=<tag or SHA>`, image digest | the same commit deploys something else tomorrow; a remote base pulls from a repo the project does not allow ([P17](attack-paths.md#p17)) | depth | platform | - |
| G6 | A stage changes by promotion: a commit made by the promotion tool. Promotion to live is a human decision. | Kargo stage with manual approval for live | change reaches live without having run in a lower stage ([P37](attack-paths.md#p37)) | depth | platform | - |

### Gate 4: Delivery

| # | Rule | How to meet it (Argo CD, Kargo) | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| D1 | The delivery system has one project per stage that fixes source repositories, target cluster and namespaces, and allows no cluster-wide kinds. The default project allows nothing. | `AppProject` with `sourceRepos`, `destinations`, empty `clusterResourceWhitelist`; `default` project emptied | merged change deploys into another stage or creates cluster-wide objects ([P37](attack-paths.md#p37)) | depth | platform | could check |
| D2 | The cluster follows git and nothing else. | automated sync with `prune` and `selfHeal` | manual change in the cluster stays unnoticed; git no longer says what runs | depth | platform | - |
| D3 | The delivery system itself has only the rights it needs. | Argo CD cluster role limited to managed namespaces; unused render tools and plugins disabled; Kargo controller reads Secrets in project namespaces only (default) | write access to a trusted repository becomes cluster-admin; a chart reads files of other repositories during rendering | depth | platform | - |
| D4 | Humans reach the delivery system by single sign-on, and who synced or promoted is recorded. | OIDC, local admin accounts disabled, Argo CD events and Kargo promotions kept | sync or promotion without a name | depth | platform | - |

### Gate 5: Cluster

| # | Rule | How to meet it (Kubernetes) | Risk without it | Level | Owner | zero |
|---|---|---|---|---|---|---|
| K1 | One namespace per service and stage. The namespace is the boundary. | `shop-dev`, `shop-qa`, `shop-live` | who can create a workload in a namespace reaches every Secret and ServiceAccount in it | depth | platform | - |
| K2 | Workloads run without host access. | Pod Security Admission `restricted` on the namespace | pod mounts host paths, runs privileged | depth | platform | could check |
| K3 | Storage is bound to the stage. Only the platform creates PersistentVolumes. | admission policy on PVCs (no `volumeName`, no cross-namespace source, storage class of the stage) | qa workload mounts the live volume ([P37](attack-paths.md#p37)) | depth | platform | could check |
| K4 | A namespace reaches only the network targets it needs. | `NetworkPolicy` default deny for egress, allow list per stage | qa workload talks to the live database ([P37](attack-paths.md#p37)) | depth | platform | could check |
| K5 | A namespace reads only the secrets of its stage. | `SecretStore` per namespace with a role for that stage; no `ClusterSecretStore` without namespace conditions; policy on the secret key prefix | qa workload fetches the live credential ([P37](attack-paths.md#p37)) | depth | platform | - |
| K6 | Each stage has its own credentials for everything outside the cluster. Live has its own cluster, or at least own nodes and storage. | separate database users, buckets, cloud roles | rules K3 to K5 are the only wall between qa and live ([P37](attack-paths.md#p37)) | depth | platform | - |
| K7 | Humans log in to the cluster with short-lived credentials and without cluster-admin for daily work. | OIDC or cloud SSO; namespace `RoleBinding`s, no wildcards | stolen kubeconfig works for months; any mistake is cluster-wide | depth | platform | could check |
| K8 | The API audit log is kept outside the cluster. | audit policy + log shipping | nobody can say who did what | depth | platform | - |
| K9 | Logs and ConfigMaps hold no secrets. | review; secret scanning of logs | a read-only agent still reads secrets ([P29](attack-paths.md#p29)) | depth | team | - |

## 5. Control files

Control files decide where and with which rights things run. An agent may read them. A change needs their owner (F4).

| File | Why it is a control file |
|---|---|
| `.gitlab-ci.yml` and everything it includes | decides what runs with the pipeline's rights |
| `CODEOWNERS` | decides who owns the other control files |
| `AGENTS.md`, skills, `.zero/policy.yaml` | instruct the agent and set its capabilities |
| Argo CD `Application`, `AppProject`, `ApplicationSet` | decide source, target cluster, namespace and allowed kinds |
| Kargo `Project`, `Warehouse`, `Stage`, promotion tasks, analysis templates | decide what is promoted and what counts as verified |
| namespace, admission policy, `NetworkPolicy`, `SecretStore` manifests | are the walls of gate 5; rule G3 puts them in the platform repository |

Not control files: values of a stage, application manifests of a stage. They are what the agent is there to change.

## 6. zero's side

What zero does so that the rules above are enough. Details in [architecture/28-target-architecture.md](architecture/28-target-architecture.md).

| # | zero does | Closes | Status |
|---|---|---|---|
| Z1 | The agent pushes only to its session branch `zero/<name>`; no delete, no other ref. | [P25](attack-paths.md#p25) | built (push gate) |
| Z2 | An agent push starts no pipeline unless the policy grants `ci.run`. | [P14](attack-paths.md#p14) | built |
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
| GitLab Free: no required approvals, no code owners (F3, F4) | a second person and file owners are not enforced | F1, F2, F5; the person who merges is the only review. A `write` path list in the push gate would keep the agent out of control files; not built |
| One cluster for all stages (K6) | a second wall behind K3 to K5 | K1 to K5 enforced by admission; own nodes for live |
| Kubernetes below 1.30 (no ValidatingAdmissionPolicy) | built-in admission policies (K3) | Kyverno or Gatekeeper |
| No promotion tool (G6) | recorded promotion | merge request per stage with F3 and F4 on the stage paths |
| Runners shared by all branches (C2) | isolation of unreviewed code | do not grant `ci.run`; then agent pushes start no pipeline (Z2) |

## 8. What zero checks

- Today `zero doctor` checks F1, F2 and C1 for the GitLab project around the current directory.
- Candidates, in this order: F4 (code owners exist and are required), F3 (approval settings), C4, C5, D1, K2, K3 and K4
  (only in namespaces that a zero policy grants to the agent), K7, G3.
- zero does not check gates 4 and 5 today. It has no access to the delivery system or the cluster.

## 9. Sources

Read on 2026-11-19. A rule that names no attack path rests on these.

| Source | Used for |
|---|---|
| [OpenGitOps principles 1.0](https://opengitops.dev/): declarative, versioned and immutable, pulled automatically, continuously reconciled | D2, G5 |
| [OWASP Top 10 CI/CD Security Risks](https://owasp.org/www-project-top-10-ci-cd-security-risks/): CICD-SEC-1 flow control, CICD-SEC-4 poisoned pipeline execution | F1, F3, F7, C1, C2 |
| GitLab: [protected branches](https://docs.gitlab.com/user/project/repository/branches/protected/), [approval settings](https://docs.gitlab.com/user/project/merge_requests/approvals/settings/), [code owners](https://docs.gitlab.com/user/project/codeowners/) | F1, F3, F4 |
| GitLab: [pipeline security](https://docs.gitlab.com/ci/pipeline_security/), [job token](https://docs.gitlab.com/ci/jobs/ci_job_token/), [ID tokens](https://docs.gitlab.com/ci/secrets/id_token_authentication/) | C1 to C6 |
| Argo CD: [best practices](https://argo-cd.readthedocs.io/en/stable/user-guide/best_practices/), [security](https://argo-cd.readthedocs.io/en/stable/operator-manual/security/), [projects](https://argo-cd.readthedocs.io/en/stable/user-guide/projects/) | G1, G5, D1 to D4 |
| Kargo: [secure configuration](https://docs.kargo.io/operator-guide/security/secure-configuration/) | D3, D4 |
| Kubernetes: [RBAC good practices](https://kubernetes.io/docs/concepts/security/rbac-good-practices/), [good practices for Secrets](https://kubernetes.io/docs/concepts/security/secrets-good-practices/), [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) | K1 to K3, K7 |
| External Secrets: [security best practices](https://external-secrets.io/latest/guides/security-best-practices/) | K5 |

Not read: NSA/CISA Kubernetes Hardening Guide, SLSA source track, Kargo and Argo CD ApplicationSet security pages.

## 10. Not in this file

- How zero confines the agent: [architecture/28-target-architecture.md](architecture/28-target-architecture.md).
- Example manifests for gates 4 and 5: [scenarios/kubernetes-hardening.md](scenarios/kubernetes-hardening.md).
- General cluster hardening that has nothing to do with stages or agents (node hardening, etcd encryption, image scanning).
