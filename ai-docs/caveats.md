# Caveats

Format: `CNN — <area> — <issue>` + impact/mitigation.

- C01 — OpenShell — All capabilities in arch doc Appendix A are assumptions, not verified. Must be proven in Sprint 01.
- C02 — Platform — Landlock/seccomp are Linux-only; macOS/Windows gaps unknown.
- C03 — OpenShell — Workspace is uploaded (copy), not bind-mounted; edits need sync back to host repo or git push from sandbox.
- C04 — OpenShell — Telemetry on by default; set `OPENSHELL_TELEMETRY_ENABLED=false`.
