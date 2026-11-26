# Tool evaluation: ways to run agents on Kubernetes, held against zero's invariants (#160)

State 2026-11-26. **Everything here is read (docs and source), nothing was run.** Sources are file paths in shallow clones taken on 2026-11-25/26: `ax/` = google/ax@0484e74, `sub/` = agent-substrate/substrate@7319f1a, `kagent/` = kagent-dev/kagent, `agent-sandbox/` = kubernetes-sigs/agent-sandbox, `openshell/` = NVIDIA/OpenShell (v0.1.8 and main). "not found" means the docs and source we read do not say; it does not mean the feature is absent.

Question: zero runs one sandbox per workspace on a laptop today. For many teams the sandboxes must run as pods. Which existing runtime could carry that without breaking Z-INV-1..6 (`docs/architecture/02-core-security-invariants.md`)?

## Verdict

| Candidate | What it is | Fit | Now? |
|---|---|---|---|
| OpenShell Kubernetes driver | the runtime zero already uses, with pods instead of podman | closest: same policy, placeholders and decision log as on the laptop; three invariants only partly | probe next |
| Agent Substrate | sandbox runtime (gVisor or microVM) with egress gateway and identity per sandbox | strong isolation and identity; egress rules stop at host and port | candidate below OpenShell or instead of it, later |
| AX (Agent Executor) | task scheduler, CLI and UI on top of Substrate | no: its server has no login and is reachable from the tasks | not now; look again at a stable release |
| kagent | agent framework and control plane on top of Substrate | partly: no login by default, rules per host only | not as the security layer |
| kubernetes-sigs/agent-sandbox | Kubernetes objects for sandboxes (template, claim, warm pool) | a building block; isolation and egress are left to the operator | only as what OpenShell's driver builds on |

## Invariants per candidate

h = holds, p = partly, n = no. Verdicts rest on what the projects write about themselves.

| Invariant | OpenShell k8s | Substrate | AX | kagent | agent-sandbox |
|---|---|---|---|---|---|
| Z-INV-1 agent not trusted as boundary | h: supervisor and kernel rules outside the agent | h: agent is "untrusted, mutually hostile"; gVisor or microVM only | n: `ax-server` without login (`ax/SECURITY.md:11-15`), reachable from tasks by issue #376; `debug: true` gives exec | p: rests on Substrate; worker hardening not settable (#3139) | p: runs the pods; gVisor/Kata must be forced by admission policy |
| Z-INV-2 capabilities, not ambient authority | p: host, port, method, path, MCP tool, per binary; but no header rules, a foreign key passes (C18) | p: default deny; host + port only, no method or path (#2541, #4140) | n: issue #376 reports that a task without gateway gets open egress and the port of a rule is not read; no tool list per task found | p: host only, every path; a namespace shares all policies | p: default rule blocks cluster-internal ranges but allows the internet |
| Z-INV-3 authenticated, authorized, attributable | h: gateway login, sandbox token, decision log | p: SPIFFE certificate per sandbox required at egress; docs overstate the checks (#3709) | n: no login on the control plane | p: default `insecure` + no authorizer; token per sandbox exists | p: router allows all by default; good access log |
| Z-INV-4 no long-lived credentials to agents | h: placeholders, short tokens | h: placeholder swapped at the gateway; but source is a static Kubernetes Secret | h: same; model key resolved outside the task | h: same as Substrate | p: only as an example with a separate proxy |
| Z-INV-5 containment survives supply-chain compromise | p: only if the cluster's network plugin enforces policies; Kata is off by default; no rule may skip TLS inspection | p: strong sandbox, but worker pods run as root with SYS_ADMIN, seccomp open (#2749, #2084) | p: as Substrate, plus the open server | p: as Substrate | p: depends on the operator's runtime class and policies |
| Z-INV-6 evidence and revocation | p: log line per decision; connections under a changed rule close; requests already forwarded are not undone | p: suspend closes connections; rule change does not cut open ones; a request log per decision: not found | n: gateway rules copied at task creation, edits do not reach running tasks (`ax/docs/manifests.md:91`) | p: next request, open ones survive | p: 30 s cache, tokens live until expiry |

## Facts per candidate

### OpenShell Kubernetes driver (v0.1.8; latest read v0.1.10)

- Creates an agent-sandbox `Sandbox` with a supervisor pod and a workload pod; both non-root, all capabilities dropped; Landlock and seccomp inside; Kata by `runtime_class_name`, off by default (`openshell/crates/openshell-driver-kubernetes/README.md:81-85,417-446`, `docs/how-it-works/sandboxes/runtimes.mdx:88-99`).
- A NetworkPolicy lets the workload reach only the supervisor; **the cluster's network plugin must enforce it** (driver README `:103-111`, `docs/kubernetes/setup.mdx:12-15`). The rules are the ones zero uses today (`network-rules.mdx`).
- Gateway login by mTLS or OIDC with roles; an unauthenticated mode exists for local use (`docs/how-it-works/gateways/authentication.mdx:34-47,143-170,373`). Sandbox tokens (lifetime not found at the place read), SPIFFE token grants (`docs/kubernetes/access-control.mdx:32-41`).
- Config is Helm values and policy files, no Kubernetes objects of its own. Quotas: not found. Image signing: not found; SBOM attestations exist.
- zero's own findings on this runtime stay valid (`ai-docs/caveats.md`): no header matching (C18), profile and policy rules add up.

### Agent Substrate (v1.0.1)

- gVisor or Cloud Hypervisor microVM; plain containers are not supported (`sub/docs/security/security.md:61`). Threat model in `sub/docs/security/threat-model.md`.
- All TCP of a sandbox goes through an Envoy egress with DNS relay; default deny; TLS opened with certificates the gateway mints (`sub/docs/guides/egress-traffic.md`, `egress-trust-bundle.md`).
- A SPIFFE certificate (1 h) per sandbox (`sub/docs/security/security.md:81`); tokens of 5 to 60 min (`sub/docs/guides/egress-credential-injection.md:162-167`).
- Egress has an `ext_proc` hook (experimental, headers only) where an outside decision service could sit (`sub/docs/security/extensibility.md:16-27`).
- Needs Kubernetes 1.36+, a policy-enforcing network plugin, a node daemon. Tenants ("atespaces") are not an isolation boundary yet; no quotas (`sub/docs/security/security.md:67,106`).

### AX (v0.4.0-rc.2, "heavy development")

- `ax-server` (gRPC, Postgres), runner as first process in the task, CLI, local UI; kinds Task, Workspace, Gateway, Model (`ax/DESIGN.md`, `ax/docs/manifests.md`).
- Its own security file says: no login on the API, tenants are not isolation, debug mode gives arbitrary exec (`ax/SECURITY.md:11-15`).
- Apache-2.0, maintained by Google. Images must be pinned by digest; signing and SBOM: not found. Logs as JSON to stdout, traces off by default; an audit record per decision: not found. Tenants as in Substrate: no isolation, no quotas.
- Clones repositories over HTTPS at a ref; push rules, branch limits, pipeline handling: not found. Approval before a wider rule: not found (roadmap).
- Local mode is a kind cluster and needs a rootful Docker or Podman (`ax/docs/local_installer.md:13-35`), which ADR 05 rules out.

### kagent (v1.0.1)

- Agent framework with CRDs; sandboxes run on Substrate. Its CNCF self-assessment says it "should not be considered a security provider" (`kagent/contrib/cncf/security-self-assessment.md:50`).
- Default control plane is `insecure` with no authorizer; login needs a proxy with OIDC (`kagent/docs/architecture/oidc-proxy-authentication.md:8-15`).
- Egress per origin, every path; tool lists per MCP server exist; approvals are documented, an approval rule per tool was not found (`egress-policies.md:45-49`, `mcp.md:85-93`).

### kubernetes-sigs/agent-sandbox (v1.0.12)

- Objects Sandbox, SandboxTemplate, SandboxClaim, WarmPool. Does not isolate by itself: the operator sets the runtime class in a `SandboxTemplate`; a bare `Sandbox` needs an admission policy to force it (`agent-sandbox/docs/security/threat_model.md:64-66`).
- Managed NetworkPolicy blocks private ranges and metadata but allows the public internet (`:65`). No credential broker built in.

### Looked at briefly

- E2B (`e2b-dev/infra`): Firecracker microVM per sandbox, egress by host name; credential handling outside the sandbox not found in the README.
- OpenHands: agent server with a container per conversation; Kubernetes, egress and identity controls not found in the README.

## What stays zero's job with every candidate

| zero's part | Candidates cover it? | Conflict |
|---|---|---|
| Sandbox | yes, all (strength differs) | AX local mode needs a rootful engine (ADR 05) |
| Egress rules per host | yes: OpenShell, Substrate, AX, kagent | none |
| Forge limited to the workspace's projects (path rules) | OpenShell only | Substrate, AX, kagent stop at the host |
| Credential outside the sandbox | yes: OpenShell, Substrate, AX, kagent | their source is a stored secret; zero mints per sandbox |
| Push gate, forge token per sandbox, policy approval | no | none: zero keeps them |
| Decision log | OpenShell yes; others traces or not found | none |
| Model and tool path (#157) | kagent and AX have their own model and MCP config | overlaps with LiteLLM / agentgateway choice |

None of them has these; zero keeps them outside the sandbox:

- the push gate (own branch only, pipeline skipped, push options refused),
- forge access limited to the workspace's projects, with a token minted per sandbox and revoked at pause and removal,
- approval before a repository's policy widens,
- the definition of agents and workspaces and what each may use.

## Recommendation

1. **Stay on OpenShell and probe its Kubernetes driver.** It is the candidate with the fewest gaps on paper (Z-INV-1, 3, 4 hold; 2, 5, 6 partly: C18, the network plugin, no undo of forwarded requests), and the laptop and the cluster would share one policy format. Phase 3 already plans this.
2. **Keep Substrate as the candidate for the sandbox layer** if OpenShell's pods turn out too weak or too slow to start: stronger isolation and an identity per sandbox, but rules only per host, so zero's path rules for the forge would need the `ext_proc` hook or a proxy of ours.
3. **Do not build on AX or kagent now.** Both leave the control plane open by default. Their scheduling, CLI and UI overlap with what zero-cp is meant to become (#157); read them again for ideas, and AX again at a stable release with issue #376 closed.
4. agent-sandbox comes in only through OpenShell's driver.

## Probes worth running (each its own issue)

1. OpenShell Kubernetes driver on a kind cluster with a policy-enforcing network plugin: `tests/attacks/forge.sh` from a pod sandbox; push gate reachable from the pod; time until a removed rule is refused.
2. The same sandbox under gVisor and under Kata: do Landlock and the seccomp filter still work.
3. Substrate alone on kind: escape and egress tests, what the egress log holds per request, whether `ext_proc` can refuse by path.
4. Whether a local kind setup works with rootless podman at all (ADR 05).

## Not shown

Nothing was run. Version numbers and issue states are as read on 2026-11-25/26. The OpenShell verdicts repeat its documentation for the Kubernetes driver; zero has only run its podman and VM drivers.
