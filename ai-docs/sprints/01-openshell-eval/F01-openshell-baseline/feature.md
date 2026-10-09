# F01 — OpenShell local install + baseline sandbox
Status: done
Description: Install OpenShell gateway + Podman driver locally; start a sandbox with a mounted repo workspace.
Goal: Reproducible one-command sandbox start with `go`, `git` available.
Plan:
1. Read OpenShell docs (install, runtimes, policy, providers); note version.
2. Install gateway + rootless Podman driver.
3. Start sandbox with a test repo mounted as workspace.
4. Script it (`scripts/sandbox-up.sh`).
DONE: sandbox starts from script; `go test ./...` and `git status` succeed inside.
GOOD: version pinned; setup steps < 10 lines in Log; teardown script exists.
Proof: see review.md step 3+4 → exit 0.
Log:
- 2026-10-09: step 1 — read docs @ v0.1.2. Pin `OPENSHELL_VERSION=v0.1.2`. Host OK (kernel 7.0, Landlock, cgroup v2) except Podman 5.x missing → blocked on install. Findings: see `notes.md`.
- 2026-10-09: step 2a — rootless Podman 6.1.3 working (user units, helper dirs, allowlist image policy).
- 2026-10-09: step 2c — OpenShell 0.1.2 installed; gateway on rootless Podman, mTLS, telemetry off.
- 2026-10-09: step 3+4 — `sandbox-up.sh`/`sandbox-down.sh`; go test + git OK as uid 1000. DONE.
