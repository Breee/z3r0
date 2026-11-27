# PoC 24 — Claude Code in a sandbox against the model proxy, no Anthropic login (#166)

Question: can Claude Code run in a zero sandbox with the model proxy as its only model address, the proxy key held outside the sandbox, and no Anthropic login?

Result (2026-11-27, Claude Code 2.1.332, LiteLLM v1.110.3, OpenShell 0.1.8, model `github_copilot/claude-haiku-4.5`): **yes.** `run.sh`: 9 PASS, 0 FAIL.

## Sources

- Claude Code: https://code.claude.com/docs/en/llm-gateway, /llm-gateway-connect, /llm-gateway-protocol, /authentication, /env-vars, /model-config, /headless
- LiteLLM: https://docs.litellm.ai/docs/providers/github_copilot

## Chain

Claude Code (sandbox; `ANTHROPIC_BASE_URL` = proxy, `ANTHROPIC_AUTH_TOKEN` = placeholder) -> OpenShell swaps the placeholder for the sandbox's proxy key (`models` rule, PoC 20) -> LiteLLM (`POST /v1/messages`) -> GitHub Copilot.

## Result

| Check | Result |
|---|---|
| Without a rule that names the program: no answer | PASS |
| Program added to the `models` rule only | PASS |
| Prompt answered through the proxy | PASS |
| Tool use: reads a file | PASS |
| Tool use: runs a shell command | PASS |
| The proxy key in the sandbox is a placeholder | PASS |
| No key in what Claude Code wrote under its home | PASS |
| api.anthropic.com not reachable from the sandbox | PASS |
| No Anthropic login variable in the sandbox | PASS |

## Findings

1. `ANTHROPIC_AUTH_TOKEN` (sent as `Authorization: Bearer`) with the placeholder works in `-p` mode without a login prompt. The base address is given without `/v1`.
2. The model name with a slash is taken as written. LiteLLM's docs say Claude Code only takes names that start with `claude-`; that did not hold here, no second name in the proxy is needed.
3. In a run by hand the proxy log showed only `POST /v1/messages`. The routes zero already lets through are enough.
4. Claude Code writes `~/.claude.json` and session files under `~/.claude`. No key is in them.
5. The egress rule must name the program, as for the Copilot CLI (PoC 23).

## Limits

- Only `-p` runs; the interactive screen was not started.
- One model, a Claude model served by Copilot. A non-Claude model was not tried; Anthropic says it does not support that behind a gateway.
- The WebFetch tool's check against api.anthropic.com was not tried (the host is not reachable).
- The PoC copies the program into the sandbox; it is not in zero's image yet.

## Run

    pocs/24-claude-code-proxy/run.sh

Needs a sandbox of a workspace that uses the model proxy (default `zero-developer`) and podman to fetch the program (or `CLAUDE=<path of the binary>`).
