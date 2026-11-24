# 28. Target architecture (onion)

Status: core built for the local version, additions PoC or planned (#94; state of 2026-11-24). Builds on §2 invariants, §27 control plane, ADR 0024 (DevOps backbone), PoCs 01–11.

## 28.1 Idea in one sentence

The agent works in a sandbox, sees no secret, can only reach what the project allows, and can only change the forge through one narrow door: its own session branch. Everything after that (review, CI, merge, deploy) is the existing DevOps/GitOps process.

## 28.2 Components (two binaries, one dependency)

| Component | Runs | Job |
|---|---|---|
| `zero` CLI | laptop | `zero up/attach/web/code/status/pause/rm/ls/switch/doctor/install` (all commands: `docs/content/docs/commands.md`); reads `.zero/policy.yaml` from the host checkout |
| `zero-cp` | laptop (user unit `zero-cp.service`) or cluster | the service (control plane): mints/renews/revokes credentials, push gate, decision log. Not a command: `zero up` starts it. Same binary in both places; the laptop instance serves one user |
| OpenShell (dependency) | laptop / cluster | gateway + supervisor: sandbox, egress policy, placeholder substitution |
| optional: OpenBao, SPIRE | cluster | credential storage off the laptop, workload identity |

Built (#97, ADR 0031; own binary since #119, ADR 0040): `zero-cp` hosts the push gate and the token upkeep loop (renew before expiry, revoke orphans); `zero up` starts it, `zero install` makes it a user unit. Creating and removing a sandbox still mint and revoke in the CLI. Not yet in it: the token endpoint for supervisor identities (PoC 09) and a central deployment.

```mermaid
flowchart LR
  dev[developer] --> cli[zero CLI]
  cli --> gw[OpenShell gateway]
  cli --> srv[zero-cp]
  gw --> sup[supervisor]
  sup --> agent[agent in sandbox]
  sup -- fetch, API, model --> ext[gitlab.com / model API]
  sup -- git push --> srv
  srv -- session branch only --> ext
  srv -. optional .-> bao[OpenBao]
```

## 28.3 Layers (inside → out)

Each layer is a feature. L0–L3 are always on; L4+ are switchable per project/org, default on once proven.

| L | Layer | Stops | Inv. | Status |
|---|---|---|---|---|
| 0 | Containment: non-root, Landlock, seccomp, no host mounts, own netns | agent touching host files/processes | Z1, Z5 | proven (PoC 01, containment 45/45) |
| 1 | Egress allowlist: deny by default, project hosts from host checkout, per-binary rules | exfil to non-allowed hosts (not through allowed ones, C18, C19) | Z2, Z5 | proven (PoC 03, `.zero/policy.yaml`); a host taken out of the file is refused after the next `zero up` (#141) |
| 2 | Placeholders: real credentials only in supervisor, injected at declared placement | secret theft from env/files/memory of agent | Z1, Z4 | proven (GitLab, Copilot #86) |
| 3 | Scoped credentials: one fine-grained token per sandbox, project-only, 24–48 h (ADR 0036), renewed, revoked on pause/rm/orphan | blast radius beyond the project; leftover tokens | Z2, Z4 | proven (#70); renew window C44 |
| 4 | Git write gate: push only to session branch, no delete | overwriting/deleting other branches (C43) | Z2, Z3 | built (#95, ADR 0030); force-push on own branch left to the repository rules (F7: new commits remove the approvals) |
| 5 | API action gate: on the forge host only GET, HEAD and POST below the workspace's projects pass (no DELETE, PUT, PATCH; no GraphQL, no other project); token has no branch/tag/commit create | deleting branches/tags, creating branches past the git gate, reading or writing other projects on the forge host, also with a foreign token (C35) | Z2 | built (#96; #52, ADR 0046), `tests/attacks/forge.sh` 45 PASS on 2026-11-23 |
| 5a | CI gate: push gate adds `ci.skip` and refuses push options; a human starts the pipeline; `gitlab.mr.create` only together with `ci.run`; with `ci.run` the pipeline starts unless the session branch changes a pipeline definition file | agent code running in CI on push (P14): CI variables, runner egress | Z1, Z2 | built (#102, ADR 0035; #134, ADR 0044; #126, ADR 0045) |
| 6 | Isolation strength: VM driver per risk (`zero up --vm`) | kernel escape | Z5 | proven (#17); no container build (C48) |
| 7 | Identity + revocation off the laptop: SPIFFE per sandbox, sessions/epochs, OpenBao storage | stolen laptop, offline revocation | Z3, Z4, Z6 | PoC 08/09/10 |
| 8 | Evidence: decision log + OCSF per sandbox → sink | silent misuse, no attribution | Z3, Z6 | partial (local logs only) |
| — | DevOps backbone: protected branches, MR review, CI, GitOps | merging agent work unreviewed | — | not zero (ADR 0024) |

Known holes no layer closes today (L0 also assumes the host follows `docs/host-recommendations.md`, e.g. no docker group): foreign credentials in requests (C18, #14; on the forge host only towards the workspace's own projects), data leaving through an allowed host (C19, #37), text from non-members on the forge read by the agent (#110), spam MRs (accepted as reversible), agent-committed hooks run on host checkout (C42), CI runs agent code once a human starts the pipeline or `ci.run` is approved (P14, [ADR 0034](../adr/0034-agent-is-untrusted-pusher.md), [ADR 0035](../adr/0035-ci-skip-by-default.md)), model traffic content. Where corruption comes from and the two candidate layers at the root: `docs/attack-paths.md`, "Origins of agent corruption" (#108). What zero trusts and what it mitigates: [ADR 0033](../adr/0033-trust-assumption-and-review.md).

## 28.3a Core and additions (2026-11-23)

Focus now: the local version (one developer, one host) with its core. Everything else is an addition layered on top: switchable, removable, and the core works unchanged without it.

| | Contains | Status |
|---|---|---|
| Core | L0–L5a, workspaces (`docs/spec/workspaces.md`), `zero-cp` on the laptop (push gate, token upkeep, decision log), GitLab through glab login, install and doctor | built; open items below |
| Additions | MCP through a gateway (ADR 0043, PoC 13–15), Kubernetes access (#123), OpenBao store (ADR 0032), SPIFFE identity (L7), VM driver (L6), evidence sink (L8), data-flow guardrail (#129), access broker for an enterprise setup (#133), team deployment (§28.5) | PoC or planned |

Open in the core: with `ci.run` and `gitlab.mr.create` a merge request opened by API runs a changed pipeline definition (ADR 0045); forge content from non-members reaches the agent unmarked (#110); foreign credentials in requests (C18, #14; on the forge host only towards the workspace's own projects since ADR 0046).

An addition names its seam to the core (sandbox identity, workspace config, session start and stop) and keeps Z-INV-1..6; example: `pocs/15-gitlab-mcp/README.md`.

## 28.4 Use (what a developer types)

```
sudo zero install          # once: OpenShell + zero-cp, one sudo
zero doctor                # checks the host, the model login and, in a repo, the forge guardrails
zero up                    # in any repo: workspace, sandbox, token, policy, push gate
zero attach                # work with the agent (or: zero web, zero code)
zero pause                 # stop the sandbox, revoke the token; zero up continues
zero rm                    # revoke + delete the sandbox
```

User guide: `docs/content/docs/` (Hugo site, #135).

Project owners add `.zero/policy.yaml` (hosts, capabilities). Widening asks the developer once (#78). Nothing else to configure. `zero doctor` checks the host setup, the model login and the forge guardrails of the repository, and names what is missing; it does not test L0–L5a themselves (`tests/attacks/` does).

## 28.5 Deployments, not modes

| | Laptop | Team |
|---|---|---|
| zero-cp | user unit on laptop | in cluster behind HTTPS |
| credential source | developer's glab/OAuth login | GitLab OAuth app, refresh token in OpenBao |
| identity | local session | SPIFFE per sandbox (L7) |
| revocation | `zero rm` / orphan sweep | central, without laptop |

The CLI, policy file and layers are identical; only where `zero-cp` runs changes.

## 28.6 What is not zero

| Not zero | Who does it |
|---|---|
| Review, protected branches, merge rules, pipelines, deployment | the forge and the team's DevOps/GitOps process (ADR 0024, `docs/gitops-manifesto.md`) |
| What a merged change does in a cluster (e.g. a lower environment pointed at live data, P37, #124) | cluster policies: NetworkPolicy, admission policies (Kyverno, OPA); zero only decides what is in the sandbox |
| The sandbox itself: isolation, egress proxy, credential placeholders | OpenShell |
| The agent and the model | opencode and the model provider; zero does not read or filter model traffic |
| Secret storage for a team, workload identity | OpenBao, SPIRE (additions) |
| Human login to the forge | glab (ADR 0038) |

## 28.7 Open points and their issues

| Open | Layer | Issue |
|---|---|---|
| Foreign credentials in requests to allowed hosts | L1 | #14 (C18) |
| Data leaving through an allowed host | L1 | #37 (C19) |
| Text from non-members on the forge reaches the agent unmarked | above L5 | #110 |
| With `ci.run` and `gitlab.mr.create` a merge request opened by API runs a changed pipeline definition | L5a | ADR 0045, no issue: accepted with the two capabilities |
| Force-push on the own session branch | L4 | none: left to repository rule F7 |
| Another host process on the gate port while `zero-cp` is down | L4 | C55, no issue |
| Token type: personal fine-grained token; service account not pursued (decision 2026-11-24) | L3 | #91 |
| GitLab MCP tools through a gateway outside the sandbox | addition | ADR 0043, not built into zero |
| Kubernetes access | addition | #123 (proof on kind), not built into zero |
| Lower environment pointed at live data by a merged change | not zero | #124, postponed |
| Systems beyond the forge (Argo CD, Kargo, Grafana), gaps G1–G9 | addition | `docs/scenarios/platform-engineer-usecases.md`, not filed |
| Data-flow guardrail, access broker | addition | #129, #133 |
| Decision log to a sink | L8 | none yet |
| Team deployment of `zero-cp` | §28.5 | #92 (use cases and flows) |
