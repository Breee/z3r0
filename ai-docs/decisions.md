# Decisions (append-only)

Format: `DNN — YYYY-MM-DD — <decision>` + 1-line reason.

- D01 — 2026-10-09 — OpenShell is the runtime substrate; Z3R0 does not build its own sandbox. Reason: arch doc §4.1, §26.
- D02 — 2026-10-09 — IDE plugins are UX only, never a security boundary. Reason: arch doc §11.3/§11.4.
- D03 — 2026-10-09 — Sprint 01 targets Linux + rootless Podman driver first; OpenCode before VS Code. Reason: arch doc §25 Phase 1.
- D04 — 2026-10-09 — Podman 6.1.3 (manual install) is required; make it work rootless. Reason: user requirement.
- D05 — 2026-10-09 — Hard requirement: agent is never root and cannot use host Podman. Reason: user requirement; proved in F02.
- D06 — 2026-10-09 — Sandbox breakout = full compromise of the gateway's host user (gateway, KEK + provider secrets, podman, ~/.ssh). Treat laptop as untrusted: run gateway+podman as a dedicated host user, only short-lived/low-value creds on laptop, authority stays server-side (MR/CI/GitOps). Reason: F03 finding, arch §14.14 "compromised workstation". Verified in F09.
- D07: Foreign-key exfil (C18) mitigated by (A) OpenShell query allowlist and/or (B) our own egress or LLM proxy that strips client auth and injects ours; upstream fix requested (ai-docs/upstream/). Deferred to F10, not deployed this sprint.
- D08 — 2026-10-09 — OpenCode runs inside the sandbox (Option A); no host plugin for tool routing. Reason: F04 PoC04 full loop works; host plugin (PoC04b) leaves every non-overridden tool, MCP, LSP on host. Needs own dev image (golang + git + opencode); official opencode image is Alpine without git/go.
- D09 — 2026-10-09 — VS Code Copilot Chat runs via Remote-SSH into the sandbox (`openshell sandbox connect N --editor vscode`), not via a host extension. Reason: PoC05 full loop (edit, commit, probe 45/0) in sandbox; a host extension cannot disable built-in read/edit/terminal tools (same gap as PoC04b).
- D10 — 2026-10-09 — GO: OpenShell 0.1.2 + rootless Podman is the Z3R0 dev-laptop sandbox. No IDE plugin/extension needed now; integration = agent inside sandbox (D08, D09). Reason: F06 summary.md; high gaps closed in Sprint 02 (dev image, no-download workflow, egress proxy F10, short-lived creds).
- D11 — 2026-10-15 — F03 closed on the httpbin PoC; real GitLab token substitution test moved to F14. Reason: placeholder mechanics proven (PoC03); GitLab-specific proof belongs with short-lived creds.
- D12 — 2026-10-15 — `zero` CLI is Go (stdlib only, `cmd/zero`, `internal/zero`); `scripts/zero` is a build shim. Reason: shell version had no locking, silent failures, password in argv; Go is the project language (arch §25 Phase 1 "Go developer laptop").
- D13 — 2026-10-16 — Phase 1 reopened with sprint 04 (F16 MicroVM, F18 sandbox build) before Phase 2 planning. Reason: arch §9.5 requires in-sandbox container build and routes hostile Dockerfiles to E3; neither OpenShell assumption is tested, and E1 blocks userns so Dockerfile builds likely need the VM driver.
- D14 — 2026-10-16 — Project name is `zero` (z3r0 = zero; CLI `zero`). Reason: user; one name for repo, CLI, docs.
- D15 — 2026-10-16 — One driver per OpenShell gateway, so zero runs two gateways (`openshell` = podman/E1, `openshell-vm` = vm/E3) and selects by risk class via `zero up --vm` / `Z3R0_DRIVER` / repo `.z3r0.yaml` (arch §6.2 C0–C4→E1/E3). Reason: 0.1.2 `compute_driver` is singular and `sandbox create` has no driver flag; separate gateways also separate policy, credstore and host user.
- D16 — 2026-10-16 — Paperclip and OpenAPPA are candidate layers above/inside zero, not replacements: Paperclip = orchestration (tasks/approvals/budget) driving an OpenShell sandbox via its SSH execution target; OpenAPPA = data-flow guardrail on tool calls inside the sandbox (opencode adapter). Neither is adopted now; evaluate after Phase 2. Reason: ai-docs/tool-eval-paperclip-openappa.md.
