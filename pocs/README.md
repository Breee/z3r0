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
| 08-spire-podman | #83: SPIRE attests each sandbox supervisor on podman (per-sandbox JWT-SVID); agent cannot reach the socket | manual, see `pocs/08-spire-podman/README.md` |
| 09-zero-cp | #90: zero-cp grants a token only to a registered sandbox SPIFFE ID; supervisor injects it, agent never sees it | `OPENSHELL_SRC=… pocs/09-zero-cp/run.sh` (needs PoC 08 environment) |
| 10-openbao | #88: gateway stores provider credentials in OpenBao (vault driver), not in its database; fails closed for new credentials/sandboxes when OpenBao is down | `BAO=… pocs/10-openbao/run.sh` |
| 11-push-service | #93: host push proxy forwards only the session branch; direct push to gitlab.com denied by OpenShell | `BIND=127.0.0.1 pocs/11-push-service/run.sh` |
| 12-openbao-broker | #98: GitLab web login gives an OpenBao identity; refresh token in OpenBao mints and revokes a fine-grained per-sandbox token | `BAO=… pocs/12-openbao-broker/run.sh` (interactive) |
| 13-agentgateway | #127: agentgateway serves MCP tools only to a registered sandbox SPIFFE ID; tools allowed per sandbox, no token in the sandbox | `OPENSHELL_SRC=… AGENTGATEWAY=… pocs/13-agentgateway/run.sh` (needs PoC 08 environment) |
| 14-k8s-mcp | #128: one shared agentgateway calls the Kubernetes MCP server with each session's own ServiceAccount token (from zero-cp, per request); no cluster credential in the sandbox | `… pocs/14-k8s-mcp/run.sh` (needs PoC 08 environment and the kind cluster of `tests/k8s`) |
| 17-litellm | #156: LiteLLM v1.110.3 runs as one container without a database; 1.36 GB image, 17-26 s start, about 630 MiB idle; with Postgres for the web UI 36 s and about 780 MiB; GitHub Copilot as backend answered (each user connects in the web UI) | `COMPOSE="docker compose" pocs/17-litellm/run.sh` |
| 18-litellm-sandbox | #158: opencode in a sandbox reaches Copilot models and MCP tools through LiteLLM with one key; model list rendered from `/v1/models`, tool list per key (needs Postgres), mail address masked, blocked word refused; findings: the key is readable in the sandbox, a key of the admin user without tool list sees every tool | `COMPOSE="docker compose" pocs/18-litellm-sandbox/run.sh` |
