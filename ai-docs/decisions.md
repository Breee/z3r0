# Decisions (append-only)

Format: `DNN — YYYY-MM-DD — <decision>` + 1-line reason.

- D01 — 2026-10-09 — OpenShell is the runtime substrate; Z3R0 does not build its own sandbox. Reason: arch doc §4.1, §26.
- D02 — 2026-10-09 — IDE plugins are UX only, never a security boundary. Reason: arch doc §11.3/§11.4.
- D03 — 2026-10-09 — Sprint 01 targets Linux + rootless Podman driver first; OpenCode before VS Code. Reason: arch doc §25 Phase 1.
