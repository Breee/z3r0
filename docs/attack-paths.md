# Attack paths

All attack paths found for zero that are not fully blocked. Numbers (P01…) are stable and never reused; a new path gets the next number, a row in the index and a row in its area.

Related: design-level threat model in `architecture/14-threat-model-and-attack-paths.md`, layers in `architecture/28-target-architecture.md`, caveats (C) in `../ai-docs/caveats.md`, catalog (A, X) in F07 `catalog.md`, attack tests in `../tests/attacks/`.

Contents:
1. [Index](#1-index)
2. [Sandbox runtime and host](#2-sandbox-runtime-and-host)
3. [Egress and credentials](#3-egress-and-credentials)
4. [Forge, CI and the developer's checkout](#4-forge-ci-and-the-developers-checkout)
5. [Gateway bind mounts and input folders](#5-gateway-bind-mounts-and-input-folders)
6. [Origins of agent corruption](#6-origins-of-agent-corruption)
7. [P14 in depth: CI is code execution outside the sandbox](#7-p14-in-depth-ci-is-code-execution-outside-the-sandbox)
8. [Mitigable with OpenShell 0.1.2 alone?](#8-mitigable-with-openshell-012-alone)
9. [Open, not tested yet](#9-open-not-tested-yet)

## 1. Index

Severity: high, medium, low, inherent (cannot be closed, only narrowed), origin (where corruption comes from, section 6).
Evidence: proven = reproduced by a test or run; reasoning = not tested.

Effort: what it takes to exploit the path, from least to most. Classified by judgement on 2026-11-17, not measured.

| Effort | Needs | Example |
|---|---|---|
| E1 | a user mistake, no attacker | secret left in an input folder |
| E2 | a corrupted agent only (one injected prompt), using what it is allowed to do | data in the query of an allowed endpoint |
| E3 | a corrupted agent plus a normal step by the developer (check out, approve, download, start a pipeline, allow an endpoint) | committed hook config runs on the host |
| E4 | a foothold or preparation outside the sandbox (process on the host, published package or image, tampered provider) | malicious Go module |
| E5 | an exploit or a bug in a dependency (kernel/container CVE, forge permission bug) | sandbox breakout |

By effort: E1 P12 P30 P31 P34 P35 · E2 P02 P04 P06 P07 P08 P09 P15 P23 P25 P26 P27 · E3 P01 P05 P10 P13 P14 P16 P18 P36 · E4 P11 P17 P21 P22 P24 P28 P29 P32 P33 · E5 P03 P19 P20.
P14 was E2 before the push gate added `ci.skip` (#102) and is E2 again where `ci.run` is approved.

| # | Severity | Effort | Path | Evidence | Section |
|---|---|---|---|---|---|
| P01 | high | E3 | Sandbox-written `.git/config` executes on host after `sandbox download` | proven | 2 |
| P02 | high | E2 | Foreign API key sent to an allowed endpoint | proven | 3 |
| P03 | high | E5 | Sandbox breakout to the gateway user | proven (blast radius) | 2 |
| P04 | high | E2 | Placeholder laundering on a bound endpoint | proven | 3 |
| P05 | medium | E3 | Absolute symlink survives `sandbox download` | proven | 2 |
| P06 | medium | E2 | Disk fill, no quota | proven | 2 |
| P07 | inherent | E2 | Data in query/path/body of an allowed endpoint | proven | 3 |
| P08 | low | E2 | Memory hog kills the sandbox | proven | 2 |
| P09 | low | E2 | Info leaks from /proc, /etc/hosts, kernel | proven | 2 |
| P10 | low | E3 | Non-HTTP / `tls: skip` endpoints uninspected | docs only | 3 |
| P11 | low | E4 | Unsigned image from an allowed registry | policy review | 2 |
| P12 | low | E1 | Same-name sandbox recreate right after delete | observed | 2 |
| P13 | high | E3 | Agent-committed hook/tool configs run on host checkout | reasoning | 4 |
| P14 | high | E3 | Agent code runs in CI on push | proven (#102) | 4, 7 |
| P15 | high | E2 | Prompt injection via forge content or dependencies | reasoning | 4 |
| P16 | medium | E3 | Agent widens `.zero/policy.yaml`, approval fatigue | reasoning | 4 |
| P17 | medium | E4 | Malicious Go module runs on host or CI | reasoning | 4 |
| P18 | medium | E3 | Agent content rendered in host browser / VS Code | reasoning | 2 |
| P19 | medium | E5 | GitLab MCP tool ignores project scope | unverified | 4 |
| P20 | medium | E5 | Token inherits the developer's role | accepted | 4 |
| P21 | low | E4 | Token not revoked after crash or host loss | by design | 3 |
| P22 | low | E4 | Local process connects to the forward port | reasoning | 2 |
| P23 | low | E2 | Resource abuse (branches, MRs, pipelines, quota) | inherent | 4 |
| P24 | low | E4 | Supply chain of zero itself | accepted | 2 |
| P25 | high | E2 | Agent changes branches other than its session branch | open | 4 |
| P26 | origin | E2 | Untrusted text the agent reads | reasoning | 6 |
| P27 | origin | E2 | Poisoned instructions in the repo | accepted | 6 |
| P28 | origin | E4 | Tool and plugin supply chain | reasoning | 6 |
| P29 | origin | E4 | Model or provider | reasoning | 6 |
| P30 | origin | E1 | The human | reasoning | 6 |
| P31 | origin | E1 | Plain model error | reasoning | 6 |
| P32 | medium | E4 | Gateway client key holder bind-mounts gateway user files | reasoning | 5 |
| P33 | medium | E4 | Gateway client key holder attaches any podman volume | reasoning | 5 |
| P34 | low | E1 | Secret in an input folder under an unknown name | by design | 5 |
| P35 | low | E1 | Input folder content carries instructions | reasoning | 5 |
| P36 | low | E3 | Repo bundle carries the stash into the sandbox | fixed #116 | 4 |

## 2. Sandbox runtime and host

P01–P12 source: F02, F03, F08 runs. P18, P22, P24: recorded 2026-11-10 for the zero workflow (scenario `scenarios/go-container-gitlab.md`), not proven.

| # | Severity | Path | Impact | Proof / status | Mitigation | Ref |
|---|---|---|---|---|---|---|
| P01 | high | Sandbox writes `.git/config` (core.fsmonitor, hooksPath, sshCommand, pager…) → user runs `sandbox download` → `git status` on host | Code exec as host user → ~/.ssh, podman, gateway secrets | `tests/attacks/run.sh` dl:git-fsmonitor-host-exec | Never copy raw `.git` back; return work via git push/MR or patch | C20, X01 |
| P03 | high | Sandbox breakout (kernel/container CVE) → host user `openshell` (since F09; before: `bree`) | as openshell: gateway db + KEK = all provider secrets, its podman. Before F09 also: ssh key+agent, gh/glab/docker tokens, podman socket; docker group = host root | `tests/breakout/blast-radius.sh` (14 findings) | Gateway + podman as dedicated user (D06, F09); VM driver; short-lived creds | C13, C25, A18, C24 |
| P05 | medium | Absolute symlink in workspace (e.g. → ~/.ssh) survives `sandbox download` | Host tools following the link read/write host files | dl:abs-symlink-kept GAP | Don't download; or strip symlinks after download | C21, X02 |
| P06 | medium | Disk fill: no quota on sandbox overlay/workspace | Host disk full (DoS) | res:no-disk-quota GAP | Quota on podman storage of gateway user (F09) | C22, C12(cat) |
| P08 | low | Memory hog → sandbox OOM-killed, no restart | Work in sandbox lost (host safe) | res:oom-kills-sandbox GAP | Push work early | C23 |
| P09 | low | Info leaks: /proc/mounts + mountinfo (username, host paths), /etc/hosts (hostname, LAN IP), kernel version | Recon for P03 | `tests/containment/run.sh`, info:* | Patch kernel; VM driver | C11, C24 |
| P11 | low | Unsigned image from an allowed registry | Malicious image runs as sandbox | policy.json review | sigstore/GPG verification | C08, C17(cat) |
| P12 | low | Same-name sandbox recreate right after delete | Stale state / failed run | observed in F02 | Unique names | C14, X07 |
| P18 | medium | Agent-generated content rendered in host browser (`zero web`) or VS Code client (`zero code`): markdown/HTML, workspace settings, extension recommendations | Exec/phishing in host UI process | not tested | — | X08, F05 |
| P22 | low | Other local host user/process connects to forward port 127.0.0.1:<port> | Needs password (0600 file in state); brute force possible, no rate limit known | not tested | — | F17 |
| P24 | low | Supply chain of zero itself: OpenShell tarballs checksum-only, uniget images `insecureAcceptAnything` (digest-pinned), local dev image | Malicious runtime/tooling | accepted for now | — | C41, ADR 0021 |

## 3. Egress and credentials

| # | Severity | Path | Impact | Proof / status | Mitigation | Ref |
|---|---|---|---|---|---|---|
| P02 | high | Agent sends its own/attacker API key (Authorization, X-Api-Key, ?api_key) to an allowed endpoint | Data exfil into attacker account on a legit service (e.g. LLM provider) | `pocs/03-credential-placeholder/run.sh` foreign-* FAIL | Query deny rules; upstream request (`../ai-docs/upstream/`); own egress gateway (F10) | C18, X10 |
| P04 | high | Placeholder laundering: agent puts placeholder in query / any other header / basic auth on the bound endpoint; upstream echoes or stores it | Real secret returned to agent | `tests/attacks/run.sh` launder:* GAP | Never bind creds to reflecting endpoints; tight path/query rules | C17, C27, X04 |
| P07 | inherent | Data in query/path/body of any allowed endpoint | Source/arch exfil | exfil:data-in-query GAP | Few endpoints, read-only, tight path/query rules, Z3R0 audit | C19, A11, X03 |
| P10 | low | Non-HTTP / `tls: skip` endpoints: no L7 check, no credential rewrite | Uninspected channel if such endpoint is allowed | docs only | Don't allow such endpoints | C15, C19(cat) |
| P21 | low | Sandbox crash or host loss without `zero rm` → token not revoked | Valid token in gateway credential store up to 1 day | by design (expiry) | — | C35 |

## 4. Forge, CI and the developer's checkout

Recorded 2026-11-10 for the zero workflow, not proven unless noted. Recorded, not necessarily handled. Scenario: `scenarios/go-container-gitlab.md`.

| # | Severity | Path | Impact | Proof / status | Mitigation | Ref |
|---|---|---|---|---|---|---|
| P13 | high | Agent commits hook/tool configs (`.pre-commit-config.yaml`, `.husky/`, `lefthook.yml`, `.envrc`, `.vscode/tasks.json`, `Makefile`, `go:generate`, tests) → developer checks out the branch on the host and runs tooling | Code exec as login user, full host blast radius | reasoning; `.git/hooks`/`.git/config` do not travel (F12) | — | C42 |
| P14 | high | Agent pushes to its session branch → pipeline starts automatically and runs agent-written code (`.gitlab-ci.yml`, tests, Makefile, scripts, Dockerfile) on a runner outside the sandbox. Detail: section 7 | Exfil of CI secrets and the repo, abuse of job token / OIDC / runner | tested 2026-11-16 (#102), see section 7 | Push gate adds `ci.skip` unless `ci.run` is approved (ADR 0035); `zero doctor` fails on unprotected project CI variables | ADR 0034, #101 |
| P15 | high | Prompt injection via GitLab content the agent reads (issues, MR comments, job logs, MCP results) or via dependencies/READMEs | Agent performs attacker's actions within its permissions | inherent, not tested | — | A07, F04, P26 |
| P16 | medium | Agent widens `.zero/policy.yaml` on a branch → developer checks it out → approves the expansion prompt without reading (approval fatigue) | Broader egress/token scope for next sandbox | design (§7a), not built | — | X06 |
| P17 | medium | Malicious Go module via allowed `proxy.golang.org` → `go.sum` committed → host or CI build runs it | Code exec in CI / on host (sandbox itself contained) | reasoning | — | P13 |
| P19 | medium | GitLab MCP tools run with user-scoped `execute_mcp_tool`; some tool may not check the project scope | Read/act on other projects of the user | unverified | — | ADR 0020 |
| P20 | medium | Token inherits the developer's role (Maintainer); fine-grained permissions are the only limit — a GitLab bug or new implicit permission widens it | Merge/settings change | accepted (ADR 0020), merge not tested | — | ADR 0020 |
| P23 | low | Resource abuse: many branches/MRs/pipelines, model quota burn | CI minutes, cost, noise | inherent | — | — |
| P25 | high | Agent pushes to, force-pushes or deletes unprotected branches other than its session branch (token not branch-scoped) | Colleague work lost or tampered | open, TODO (C43) | — | C43 |
| P36 | low | A repo folder is added while the agent runs; the bundle in `/tmp/.zero.bundle` was built with `--all` and carried `refs/stash` (a `git stash -u` holds untracked files such as `.env`); the agent copies the bundle before it is deleted | Stashed secrets readable by the agent | bundle content shown on the host (#116), sandbox side not run | fixed #116: bundle holds branches, tags and HEAD only | #116, spec §6.1 |

## 5. Gateway bind mounts and input folders

Added 2026-11-17 (#77, ADR 0039). Not proven unless noted. The agent itself gains no path: the mount is read-only for a non-root user (proven: write, delete, chmod, remount fail) and it cannot ask for mounts without the gateway client key.

| # | Severity | Path | Impact | Proof / status | Mitigation | Ref |
|---|---|---|---|---|---|---|
| P32 | medium | A host process with the user's gateway client key creates a sandbox with a bind mount of any path the gateway user can read (`--driver-config-json`), also read-write | Reads the gateway user's home: gateway TLS keys, stored GitLab and Copilot credentials. Before ADR 0039 the user could not read them | not run; follows from `enable_bind_mounts = true` | Upstream read-only upload (#112), then turn the three settings off again (#111) | C52, ADR 0039, #113 |
| P33 | medium | Same process attaches any existing podman volume (`resource_admission.enabled = false`) | Reads or changes another sandbox's workspace | not run | as P32 | C52, ADR 0039, #113 |
| P34 | low | Secret in an input folder under a name the pattern list does not know (`secrets.yaml`, `prod.tfvars`, a key inside a `.txt`) | Secret readable by the agent | by design: only names are matched | `.zeroignore`; summary on `zero up`; list widened and content scan decided against in #72 | #72, spec §6.1 |
| P35 | low | Input content (specs, notes) carries instructions for the agent | Prompt injection, as P26 | not run | none; input folders are chosen by the developer | P26 |

## 6. Origins of agent corruption

Added 2026-11-16 (#108). Reasoning, not tested. Every path in sections 2–5 starts with an agent that does something its developer did not ask for. This section lists where that comes from. The model cannot reliably tell data from instructions, so the root cannot be closed; it can only be made smaller.

**Rule.** Never give one sandbox all three: untrusted input, access to secrets or private data, and an outbound channel. zero removes secrets (L2, L3) and most outbound channels (L1, L4, L5, 5a). Untrusted input is not limited by zero today.

| # | Origin | Example | Today | Gap |
|---|---|---|---|---|
| P26 | Untrusted text the agent reads (prompt injection) | issue or MR comment, web page, dependency README or source, log line, MCP tool result | egress allowlist limits what can be fetched (L1); GitLab MCP is limited to the project by the token | content from non-members of the project reaches the agent unmarked (P15) |
| P27 | Poisoned instructions in the repo | `AGENTS.md`, skills, `.opencode/`, MCP config changed on a branch | MR review before merge (DevOps backbone) | accepted (ADR 0041): the agent loads these from the checked-out branch; which branch, and review, are the DevOps process's job. zero's limits (L1–L5, push gate) apply regardless |
| P28 | Tool and plugin supply chain | MCP server, opencode plugin, skill, npm or Go module | dev image pinned by digest; registries only if allowed in `.zero/policy.yaml` | plugins and MCP servers named in repo config are not checked; modules are not vetted (P17) |
| P29 | Model or provider | backdoored model, tampered endpoint | model hosts are an explicit allowlist (zen, Copilot) | no approved-model list; model traffic content is not inspected |
| P30 | The human | careless or malicious prompt | accountability: every token and push carries the developer's identity; policy widening needs approval (#78) | none planned; this is the developer trust model |
| P31 | Plain model error | wrong command, wrong branch, deletes work | blast radius only: sandbox, session branch, no deletes | none possible at the root |

### Candidate layers at the root

Not built. Decision 2026-11-16: record it, build later; follow-up issue #110. Trusted instructions (P27, #109) were dropped on 2026-11-18 (ADR 0041).

**Trusted instructions (P27) — not built, ADR 0041.** Agent configuration (`AGENTS.md`, skills, `.opencode/`, MCP config) would be taken from the protected default branch, not from the working branch. Rejected: which branch the agent runs on, and review of repo instructions, belong to the DevOps process.

**Input provenance (P26).** Forge content written by people who are not project members is not handed to the agent, or is handed over marked as untrusted.
- Mechanism: a filter between the agent and the forge API/MCP that looks up the author of each issue, comment and MR description and drops or labels content from non-members.
- Unknowns: where the filter runs (OpenShell rules cannot rewrite response bodies; it would be a proxy in `zero-cp` or an MCP wrapper); whether labelling changes model behaviour at all; cost of the membership lookups. OpenAPPA covers part of this inside the sandbox (see `../ai-docs/tool-eval-paperclip-openappa.md`, not adopted).
- Cost: another parser of forge responses on the host; does nothing for web pages, dependencies or logs.

## 7. P14 in depth: CI is code execution outside the sandbox

Added 2026-11-16 (#101). From GitLab documentation and reasoning; the test of 2026-11-16 is noted below.

**Why it matters.** Everything else the agent does stays inside the sandbox or passes a zero layer. CI does not: a push to the session branch (allowed by design, L4) starts a pipeline with no human in between, and the pipeline runs whatever the branch contains. No exploit is needed.

**Why the developer trust model does not apply.** GitLab lets every Developer edit the pipeline because a human who sends all CI variables to a remote server is unlikely and accountable. A corrupted agent has neither property: it does it at once, whenever an injected prompt says so. The agent's push must be treated as untrusted input to CI, like a fork MR from a stranger, although it carries the developer's identity (ADR 0034).

**Forbidding edits of `.gitlab-ci.yml` does not close it.** The existing pipeline already runs agent-written tests, Makefiles, scripts and Dockerfiles.

| Reachable from the job | Severity | Note |
|---|---|---|
| CI variables not marked protected (project, group, inherited) | high | masking only hides them in the log |
| Unrestricted egress from the runner | high | bypasses the sandbox allowlist (L1): whole repo can leave |
| OIDC id tokens (cloud, OpenBao) | high where used | if the trust policy has no branch condition |
| Job token | medium | read project, write its package/container registry, projects that allowlist it |
| Code execution on the runner | low on gitlab.com shared runners, high on self-hosted (docker socket, privileged, internal network) | |
| Compute minutes | low | |

Not reachable: protected variables, protected environments, merge.

**Tested 2026-11-16 (#102, gitlab.com, self-hosted group runner):** a push without `ci.skip` ran the agent's job automatically; it read an unprotected CI variable, not a protected one, had `CI_JOB_TOKEN` and open internet. With `ci.skip` the pipeline is recorded as `skipped`; the agent token cannot start one (REST create/retry/trigger/schedule/MR pipeline, GraphQL: all refused). Built: push gate adds `ci.skip` unless capability `ci.run` is approved (ADR 0035); `zero doctor` fails on unprotected project CI variables. Not built: check of inherited group variables and runner scope.

**Candidate layers (status before #102).**

| Layer | Closes | Status |
|---|---|---|
| GitLab: variables protected, runners only for protected branches | secrets, self-hosted runner | GitLab feature; `zero doctor` check not built |
| No automatic pipeline for agent pushes: gate adds `ci.skip`, human starts the pipeline from the MR after reading the diff | everything, as long as the human reads | untested; agent may be able to start a pipeline through the API |
| Pipeline config from another project | edits of the pipeline file only | untested on gitlab.com Free; scripts still run |
| Separate untrusted runner for `zero/*` branches (no secrets, egress allowlist) | secrets, egress | idea; needs runner admin |

## 8. Mitigable with OpenShell 0.1.2 alone?

Covers P01–P12 only. yes = OpenShell policy/config closes it · partial = narrows it · no = needs host setup, workflow, or upstream change.

| # | OpenShell today | How / why not |
|---|---|---|
| P01 | no | `sandbox download` copies `.git` as-is. Workflow fix: push from sandbox, never download repos. |
| P02 | partial | Query deny rules block `?api_key=`-style keys. No header matchers → foreign Authorization/X-Api-Key pass (upstream request / F10). |
| P03 | no | Host fix done: dedicated user `openshell` (F09). Residual: KEK next to db (C13), client key in login user (C26). VM driver unknown (F09 step 4). |
| P04 | partial | Tight method/path/query rules on bound endpoints shrink it; placeholder still resolved in any header/basic auth, responses not redacted. |
| P05 | no | Download keeps absolute symlinks. Workflow fix: don't download. |
| P06 | no | No disk limit found in sandbox settings. Host fix: quota on podman storage (F09). |
| P07 | partial | Few endpoints, read-only, path/query rules. Never fully closable. |
| P08 | partial | cgroup limits protect host (default 4 GiB/2048 pids/2 CPU); sandbox still dies. |
| P09 | no | Mount/hosts info comes from podman; kernel shared. |
| P10 | yes | Don't allow `tls: skip` or non-HTTP endpoints in policy. |
| P11 | no | Image trust is podman `policy.json` (C08), not OpenShell. |
| P12 | no | Use unique sandbox names (workflow). |

Unverified: "no" for P06/P09 means not found in 0.1.2 docs/CLI, not proven impossible.

## 9. Open, not tested yet

Prompt injection via tool output (A07, F04), social-engineered rule approval (X06, F04), VS Code server phoning home (X08, F05), VM driver (C13, F09).
