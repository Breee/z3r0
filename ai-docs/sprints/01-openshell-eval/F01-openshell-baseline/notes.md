# F01 — OpenShell notes (v0.1.2, docs in repo `docs/`)

- Version: latest stable `v0.1.2` (2026-09-28). Install: `curl -LsSf https://raw.githubusercontent.com/NVIDIA/OpenShell/main/install.sh | OPENSHELL_VERSION=v0.1.2 sh` (deb on Ubuntu).
- Telemetry on by default; disable with `OPENSHELL_TELEMETRY_ENABLED=false` on gateway.
- Host reqs (support-matrix.mdx): Linux 6.2+ with Landlock ABI≥3, seccomp user-notify, Podman 5.x + user socket + cgroup v2 (or Docker).
- Model: gateway (control plane) → supervisor (trusted, outside workload) → sandbox. Network deny by default, per-binary endpoint rules.
- Lifecycle: `sandbox create --from <img> [--provider X] -- <cmd>`, `sandbox exec -n N -- cmd`, `sandbox connect N`, `sandbox upload`. Workspace = upload (copy), not bind mount → check sync-back in F01/F04.
- Providers: `profile import` + `provider create`; creds stored in gateway, injected only for bound endpoint + binary. Plain `--env` is visible to agent.
- Policy advisor: denials → proposals → `openshell rule get|approve|reject`; hot reload.
- OpenCode (F04): official example runs OpenCode **inside** the sandbox (`--from ghcr.io/anomalyco/opencode:latest -- opencode`). Likely no plugin needed for execution.
- VS Code (F05): `sandbox create|connect --editor vscode` → Remote-SSH into sandbox; best-practices mentions `/sandbox/.vscode-server/**` binary globs. Likely no extension needed for execution.
- Extensibility: supervisor middleware, gateway interceptors, compute drivers (F03/Phantom later).
- SDKs: Python, TS, Go, Rust (exec, forward, SSH, policy).
