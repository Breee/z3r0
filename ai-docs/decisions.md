# Decisions (append-only)

Format: `DNN — YYYY-MM-DD — <decision>` + 1-line reason.

- D01 — 2026-10-09 — OpenShell is the runtime substrate; Z3R0 does not build its own sandbox. Reason: arch doc §4.1, §26.
- D02 — 2026-10-09 — IDE plugins are UX only, never a security boundary. Reason: arch doc §11.3/§11.4.
- D03 — 2026-10-09 — Sprint 01 targets Linux + rootless Podman driver first; OpenCode before VS Code. Reason: arch doc §25 Phase 1.
- D04 — 2026-10-09 — Podman 6.1.3 (manual install) is required; make it work rootless. Reason: user requirement.
- D05 — 2026-10-09 — Hard requirement: agent is never root and cannot use host Podman. Reason: user requirement; proved in F02.
- D06 — 2026-10-09 — Sandbox breakout = full compromise of the gateway's host user (gateway, KEK + provider secrets, podman, ~/.ssh). Treat laptop as untrusted: run gateway+podman as a dedicated host user, only short-lived/low-value creds on laptop, authority stays server-side (MR/CI/GitOps). Reason: F03 finding, arch §14.14 "compromised workstation". Verified in F09.
- D07: Foreign-key exfil (C18) mitigated by (A) OpenShell query allowlist and/or (B) our own egress or LLM proxy that strips client auth and injects ours; upstream fix requested (ai-docs/upstream/). Deferred to F10, not deployed this sprint.
