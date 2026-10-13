# PoCs

Small proofs, one per feature. Each script prints `PASS` / `FAIL` / `GAP` / `INFO` lines,
exits 0 when all checks pass, and cleans up after itself (except 05).

Prereqs: host setup from `ai-docs/sprints/01-openshell-eval/F01-openshell-baseline/review.md`.

| PoC | Proves | Run |
|---|---|---|
| 01-openshell-baseline | F01: rootless sandbox, non-root, `go test` passes | `pocs/01-openshell-baseline/run.sh [name]` |
| containment | F02: hostile-agent probes are blocked | `tests/containment/run.sh` |
| 03-credential-placeholder | F03: env holds a placeholder; proxy injects the real secret only at the bound endpoint (needs internet) | `pocs/03-credential-placeholder/run.sh` |
| 04-opencode-in-sandbox | F04: OpenCode inside the sandbox does edit → go test → commit | `pocs/04-opencode-in-sandbox/run.sh [name] [model]` |
| 04-opencode-in-sandbox (attack) | F04: agent runs containment probe + prompt injection via `AGENTS.md`; GAP C27 | `pocs/04-opencode-in-sandbox/attack.sh [model]` |
| 04b-host-plugin | F04: host OpenCode, plugin routes `bash` into sandbox; `read` still on host (GAP) | `pocs/04b-host-plugin/run.sh [model]` |
| 05-vscode-remote | VS Code Remote-SSH into sandbox, Copilot egress scoped to vscode-server, pty works. Leaves sandbox running | `pocs/05-vscode-remote/run.sh [name]` + manual Copilot prompts |
| 11-dev-image | F11: dev image has go/opencode/git/rg on PATH, no setuid, non-root. Needs `scripts/build-dev-image.sh` | `pocs/11-dev-image/run.sh [name]` |
