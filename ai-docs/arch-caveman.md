# Z3R0 architecture — caveman version

Compressed from `Z3R0_ARCHITECTURE_v0.9_OPEN_SHELL.md` (v0.9, 2026-10-04). For agents. Section numbers point into the full doc.

## One sentence

Agent is a developer that may get hijacked at runtime. OpenShell contains it. Z3R0 decides what it may do. GitLab/K8s/GitOps keep their own guards. Nothing trusts the agent.

## Premise (§1, §2)

- Agent benign by default, corruptible by any input: issues, READMEs, logs, MCP output, deps, Dockerfiles, plugins, model.
- Question: what can a fully corrupted agent do with its current capabilities?
- Answer must come from external enforcement, never from "model notices attack".
- Invariants: Z1 agent process is not a boundary. Z2 capabilities not ambient authority (operation is the capability, not the CLI). Z3 every side effect: who/what/resource/constraints/approval/evidence. Z4 no long-lived creds in env/files/argv/history/config/logs. Z5 containment survives supply-chain compromise (child procs, tests, Dockerfiles). Z6 evidence + remote revocation are part of authz.

## Primitives (§3)

Identity (SPIFFE/SPIRE + Z3R0 session/epoch) → Capability (semantic: `git.push`, `k8s.pod.logs`, `container.build`) → Boundary (OpenShell; native GitLab/K8s/Cilium/Kyverno/Tetragon/OpenBao/LiteLLM stay authoritative) → Evidence (OTel + OCSF + Langfuse + audit).

## Ownership split (§4)

- OpenShell owns: sandbox lifecycle, fs/proc/syscall isolation, network enforcement, provider credential placeholder substitution, supervisor proxy, middleware, policy hot-reload, compute drivers (Podman/MicroVM/K8s), runtime security events.
- Z3R0 owns: human-accountable logical sessions, security epochs, semantic capabilities, approvals + one-shot grants, Organization Context/risk, Git/GitLab/K8s/container opinions, prod diagnostic policy, commit provenance, cross-system audit.
- Phantom Proxy = concept, not a binary. Built from OpenShell provider substitution + supervisor proxy + Z3R0 middleware (K8s spec projection, log sanitization, connection-string redaction, secret detectors, cumulative disclosure tracking).
- Required: OpenShell, SPIFFE/SPIRE, Z3R0 control plane, capability policy, Phantom Proxy, credential authority (OpenBao preferred), LiteLLM, Langfuse+OTel+audit. K8s RBAC/admission required for K8s scenario. Cilium/Tetragon recommended. Org Context optional.

## Capability model (§5)

Namespaces: `fs.*`, `process.exec`, `network.connect|listen`, `git.read|commit|push|force_push|tag.create`, `gitlab.issue.read|pipeline.read/run/retry/cancel|mr.read/create/merge|release.create`, `k8s.resource.read/create/patch/delete|pod.logs|pod.exec|workload.restart|secret.read|rbac.modify|crd.modify`, `container.image.pull|build|run|network|mount|device|registry.push`, `model.invoke|mcp.tool.call|a2a.agent.call`.
Resource scope: git = repo+ref; k8s = cluster/ns/apiGroup/kind/name; network = scheme+host+port. Shells/interpreters = arbitrary code inside sandbox; don't parse shell for security.

## Risk × isolation (§6)

Risk: C0 read (auto) · C1 local mutation (auto in sandbox) · C2 reversible remote (feature push, MR, DEV/QA mutate; policy + short-lived authority) · C3 protected transition (agent requests, human/delivery system performs) · C4 prod/security mutation (never normal agents; break-glass identity only).
Isolation: E1 rootless Podman (normal dev) · E2 hardened container/build sandbox (local container build/run, integration tests) · E3 MicroVM (untrusted repos, hostile builds) · E4 K8s driver (fleet/remote). Z3R0 picks profile by policy; OpenShell gives one API across them.

## Policy (§7)

One semantic Z3R0 policy (agent profile, workspace repos, capabilities, constraints like `git.push branches: feature/* deny_protected`) compiled into: OpenShell fs/proc (Landlock/seccomp), OpenShell network (dest/port/binary/L7), OpenShell providers, OpenShell middleware, LiteLLM, credential authority (role/scope/TTL), GitLab, K8s, Cilium/Kyverno/Tetragon. Native controls first. Reuse OpenShell Policy Advisor/Prover as low-level check under Z3R0 approval. Fail closed: no match = deny; policy/cred/proof failure never expands authority.

## Credentials (§8)

- Agent asks for use of authority, not the secret. Placeholder in sandbox → real cred substituted at outbound boundary after policy check. OpenBao behind that layer.
- Bind creds to identity+destination+protocol+operation+resource+TTL (no laundering a GitLab header to evil.example).
- Phantom Proxy invariant: neither devs nor sandboxes get real secrets. Order: request → policy → cred replace → target → raw → structured redaction → safe response. Can't isolate the secret part → redact whole value.
- Routing: authenticated access (GitLab, K8s, registry, cloud, DB) MUST go through Phantom Proxy. Public unauthenticated may go direct, still under egress policy. Optional public-content scanner = defense-in-depth, can only tighten.
- CLI helpers (git/glab/kubectl) must not hand back a reusable raw credential.

## Runtime (§9)

- Everything in sandbox is assumed compromised. Supervisor is outside the agent tree; Z3R0's authoritative approval/cred/sanitize logic lives there, never inside.
- Kernel controls: Landlock, non-root, zero caps, no_new_privs, seccomp, supervisor-mediated net/DNS.
- Default mode: arbitrary local processes allowed inside sandbox (bash, make, go test, python, node). Optional strict mode = command allowlist, defense-in-depth only.
- Workspace: isolated worktree/copy/volume preferred; host bind mount only as explicit weaker convenience.
- Network: no unrestricted path. Auth service → Phantom path; trusted mirror → allow; approved public → allow/scan; unknown → deny. MicroVM: force guest traffic through supervisor, not a normal NIC.
- Containers: never the dev's Docker socket. Build/run inside/through the sandbox: rootless, isolated image storage, restricted context, no host mounts/network/privileged/devices, resource limits, controlled registry access, no raw build secrets. Hostile Dockerfile → MicroVM profile.
- Integration tests on isolated test network; don't expose agent-started services on host.
- Dependencies are their own capability class: GOPROXY/npm/pip/OCI/apt via approved internal mirrors; direct public = explicit risk acceptance.

## Topology (§10)

Central: Z3R0 control plane (sessions/approvals/risk/policy/Org Context), LiteLLM, OpenBao, SPIRE server, audit/OTel/Langfuse/SIEM.
Laptop: IDE/OpenCode + Z3R0 plugin, OpenShell gateway, Podman (optional MicroVM), Z3R0 middleware, signed policy cache, telemetry forwarder. Plugin is not the boundary.
K8s/remote: OpenShell gateway + K8s driver; native RBAC/admission/Cilium/Tetragon authoritative; no mandatory extra cluster proxy unless a real gap.
Offline: already-authorized local work continues; new remote authority/epochs/approvals fail closed. No prod-writer agent in v1.

## Laptop UX (§11)

Should still feel like `git clone; code .; agent; go test; docker build; glab pipeline status; git push`. Developer shell and agent session are separate principals on the same laptop; human's ambient creds never leak to agent. OpenCode: Z3R0 plugin (session, repo/branch discovery, profile, status, trace) + custom LLM endpoint → LiteLLM. VS Code: same via custom endpoint + light extension. OpenShell hosts normal tools (git, glab, kubectl, go, docker-compatible CLI). Profile auto-derived from repo/branch/language (e.g. go-developer: workspace rw, Go tooling, feature push, GitLab read, local build/run, DEV/QA, PROD read; no protected write, no PROD mutate, no host creds, no arbitrary egress). Routine ops silent; nag dialogs train bypass.

## Reference scenarios (§12, §13)

- K8s operator: starts read-only (resource.read, pod.logs; cluster+ns scoped). No exec/delete/patch/secret.read/rbac. Prod = read-only to normal agents; fixes go feature branch → DEV/QA → CI → MR → GitOps. DEV/QA writes allowed per policy. Prod automation = separate narrow system identity; break-glass is human.
- Go dev: caps git.read/commit/push(feature/*), gitlab issue/mr/pipeline read + mr.create, container pull/build/run; network allow gitlab.com, proxy.golang.org, sum.golang.org, registry. Denied: push main, force_push, tags, pipeline.cancel, release, registry.push, host mounts, privileged. `docker build` is interpreted as `container.build` with context/Dockerfile/registry/network/secret checks → Podman/build runtime. Build secrets via Phantom Proxy short-lived mount, never `ARG GITLAB_TOKEN`. container.run defaults: no privileged/host net/pid/devices/mounts/socket, drop ALL, isolated network, cpu 2 / mem 1Gi / pids 256. `git push HEAD:main` → DENY before auth is delivered. Loop: code → test → build → integration → commit → push → CI → glab status → fix → MR.

## Threats (§14)

Prompt injection→cred exfil (fs isolation + cred replace + egress deny). Injection→legit secret API (`k8s.secret.read` absent). Injection→destructive prod (least priv + approval + break-glass + server-side authz). Malicious commit (CI/MR/CODEOWNERS; runtime just blocks bypass). GitOps poisoning (MR/CI/GitOps). Malicious MCP output (runtime still denies). Malicious tests/builds (child procs inherit sandbox). Malicious Dockerfile (isolated builder, restricted context/egress, no socket, VM for untrusted). Supply chain (DevOps controls + runtime fallback). Architecture/source exfil (egress + future data-classification per model/provider). Agent disables controls (controls outside its reach). Cred laundering (dest+op+resource bound). Confused deputy A→B copy (future info-flow labels). Compromised workstation (no long-lived secrets, short identity, remote revoke).

## Control planes (§15–§17)

LiteLLM = single AI control plane: provider keys off laptops, model allowlists, budgets, MCP/A2A routing, tool visibility/trust. "Visibility is not authorization." agentgateway optional for east-west. K8s: capability policy AND short-lived identity AND RBAC AND admission; read live, change via GitOps. GitLab: protected branches, approvals, CODEOWNERS, CI, runner isolation; agent reads issues/MRs/pipelines, pushes feature branches, creates MRs; never push main/force/merge protected/cancel prod pipelines/releases/settings.

## Evidence + revocation (§18, §19)

Every session has trace id. Chain: human → session → model call → tool → capability decision → approval → cred use → target → result. Log identity, capability, resource, decision, denial reason, approver, lease id/TTL (no secret), process, destination, request metadata, status, tokens/cost. Revocation: disable identity, revoke OpenBao leases, disable LiteLLM key, block capability, destroy sandbox/VM, revoke device, stop cluster workload, kill A2A task. Measure time-to-effect; drill it.

## DevOps vs runtime (§20)

DevOps primary: source changes, GitOps manifests, deps, promotion, protected branches, provenance, CI, review. Runtime primary: cred theft, exfil, arbitrary fs, hostile tests/Dockerfiles, injected destructive CLI/API, direct prod access, compromised plugins/workstation. Neither replaces the other.

## Session & identity (§22)

- One session abstraction for coding/operator/CI/autonomous agents. Session = actors + accountable human(s) + execution location + agent implementation + workspace + baseline caps + epoch.
- ≥1 accountable human, fixed for session life. Humans auth via enterprise IdP (OIDC/SAML); claims snapshotted at creation.
- Bound to one execution location and one agent client (OpenCode / VS Code / Claude Code / K8s agent); switching = new session. Stolen artifacts not portable.
- Workspace = set of repos only, no authz semantics. Agent can't redefine it; out-of-scope repos need request + approval. Verify repo by remote identity, not path.
- Branches are dynamic context; evaluated per op. push main/protected → DENY always.
- Logical session (resumable task context) vs security epoch (short-lived; new on every resume: re-auth human/device/runtime, re-evaluate policy, mint sender-constrained creds). Elevated/one-shot grants don't survive epochs. One active epoch per session.
- SPIFFE = runtime identity (`spiffe://corp/device/laptop-42`); Z3R0 token = session+epoch+humans+agent+workspace+expiry, sender-constrained to SPIFFE id. No bearer-only tokens.
- Baseline caps evaluated centrally at epoch creation, signed, enforced locally without round trips.
- Growth always needs human approval. In-scope extension (e.g. add related repo) may last the session. Out-of-scope = one-shot: one op, one resource, max_uses 1, short expiry.
- Prod: no access by default, not even read. Every prod op = out-of-scope one-shot. No prod mutation in v1.
- Request contents: exact capability + resource, reason, expected result, data_exposure (sensitive reads), blast_radius (mutations; agent-provided = untrusted). UI shows plain text + exact scope.
- Org Context (service map, owners, criticality, deps, classification) enriches risk, never grants. Sources tagged authoritative / observed / declared.
- Risk: deterministic first; cheap trusted model only for ambiguous cases; model may escalate only, never de-escalate or override deny.
- Prod diagnostics sanitized through Phantom Proxy before reaching laptop/sandbox; structured redaction, whole-value fallback; model detectors may add redactions only.
- Approver determined centrally; accountable human ≠ automatic approver. Lifetime profile-dependent (interactive = epoch w/ max, CI = job, autonomous = bounded renewal). Liveness policy per capability (local none, gitlab_write 2m grace, prod 0).
- Revocation: soft (stop new creds/grants/renewal; local work continues), hard (kill epoch, agent tree, builds, creds, network, grants), quarantine (hard + freeze workspace for forensics). Suspected corruption → hard/quarantine.
- Dependency/public egress: mirrors preferred; Z3R0 injects GOPROXY/npm/pip/OCI/apt config.
- Exec modes: default = arbitrary local exec in sandbox; strict = allowlist, optional.
- Workspace fs: isolated worktree/copy preferred; direct mount = convenience, weaker.
- Git provenance: agents on non-protected branches only; every agent commit signed via `gpg.format=x509` + SPIFFE Workload API program using the epoch X.509-SVID (`spiffe://corp/z3r0/epoch/ep_456`); no long-lived key on disk. Tags/releases out of scope v1.
- Repo instructions (AGENTS.md etc.), MCP output: untrusted, may guide style, never expand authority. LiteLLM exposes only permitted MCP tools.
- Phantom Proxy transport v1: HTTP(S) only (GitLab, K8s API, OCI, LLM, MCP/HTTP). Implementation = network isolation + forced proxy routing; selective TLS termination only where inspection/sanitization needed; no host-wide MITM trust root.

## Derived rules (§23, 31 of them, the ones that bite)

Assume corruption. Agents = developers. Don't rebuild OpenShell/GitLab/K8s/GitOps. Capability = operation, not binary. Authz before auth is delivered. Runtime controls cover child procs and build code. No Docker socket. Prod mutation out of scope v1. Tool output/repo content untrusted. Egress is a security control. Creds bound to dest+op. Agent can't disable its own controls. Visibility ≠ authorization. Plugins = UX. Audit + revocation mandatory. No real secrets anywhere agent/dev can reach. Scanners/risk models only tighten. Deps = own capability class. Arbitrary exec inside sandbox is fine; sandbox is the boundary. Signed commits mandatory. Phantom Proxy HTTP-first via OpenShell primitives. OpenShell is the substrate; Z3R0 doesn't build its own sandbox.

## Open questions (§24, 23 items)

OpenShell UX prototype · security fit per platform · Provider↔OpenBao · Phantom middleware · GitLab adapter · K8s adapter · prod diagnostic path · container workflow w/o socket incl. hostile Dockerfiles · MicroVM threshold · K8s driver · Advisor/Prover integration · SPIFFE git signing · epoch identity vs OpenShell identity · LiteLLM capability→tool visibility · Org Context schema · risk engine · approval UX · revocation drills · workspace semantics · mirror injection · public scanner · platform prerequisite baseline · OpenShell support matrix.

## Rollout (§25) and mapping to ai-docs/phases.md

Arch Phase 0 feasibility (12 items): 1 no host secrets/unrelated repos · 2 child procs contained · 3 unknown egress denied · 4 placeholders not extractable · 5 OpenBao GitLab creds substituted · 6 feature push ok / protected denied · 7 glab pipeline status narrow · 8 container build/run w/o socket · 9 hostile tests/Dockerfiles blocked · 10 middleware sanitizes K8s specs/logs · 11 OCSF/OTel correlation · 12 hard revoke within budget.
Arch Phase 1 = Go laptop loop on OpenShell+Podman (code → go test → build/run → signed commit → push feature → glab status → fix → MR), OpenCode first. Arch Phase 2 = K8s operator. Arch Phase 3 = MicroVM profiles. Arch Phase 4 = K8s compute driver.
ai-docs mapping: ai-docs Phase 1 = arch Phase 0 items 1–4, 9. ai-docs Phase 2 = items 5–8, 11, 12 + arch Phase 1. ai-docs Phase 3 = arch Phase 2 + item 10. ai-docs Phase 4 = arch Phase 3. ai-docs Phase 5 = arch Phase 4.

## Appendix A — OpenShell assumptions we depend on

sandbox/supervisor split · Landlock fs · non-root/no-caps/no_new_privs/seccomp · supervisor network policy (dest/binary/L7) · provider profiles with placeholders + endpoint-bound substitution · supervisor middleware that can inspect/modify traffic · Podman, MicroVM, K8s drivers · dynamic policy updates · OCSF observability · Policy Advisor/Prover. Track OpenShell versions; don't copy mechanisms into Z3R0.
