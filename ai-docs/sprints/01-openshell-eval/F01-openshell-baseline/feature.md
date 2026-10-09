# F01 — OpenShell local install + baseline sandbox
Status: doing
Description: Install OpenShell gateway + Podman driver locally; start a sandbox with a mounted repo workspace.
Goal: Reproducible one-command sandbox start with `go`, `git` available.
Plan:
1. Read OpenShell docs (install, runtimes, policy, providers); note version.
2. Install gateway + rootless Podman driver.
3. Start sandbox with a test repo mounted as workspace.
4. Script it (`scripts/sandbox-up.sh`).
DONE: sandbox starts from script; `go test ./...` and `git status` succeed inside.
GOOD: version pinned; setup steps < 10 lines in Log; teardown script exists.
Proof: `scripts/sandbox-up.sh && openshell exec … go test ./...` → exit 0.
Log:
- 2026-10-09: step 1 — read docs @ v0.1.2. Pin `OPENSHELL_VERSION=v0.1.2`. Host OK (kernel 7.0, Landlock, cgroup v2) except Podman 5.x missing → blocked on install. Findings: see `notes.md`.
