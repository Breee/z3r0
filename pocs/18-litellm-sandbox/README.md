# PoC 18 — opencode in a sandbox through LiteLLM: models, tools per key, guardrail (#158)

Question: can a sandbox get its models and its MCP tools through one LiteLLM proxy with a key of its own, with the model list taken from LiteLLM and the tool list set per key?

Result (2026-11-25, LiteLLM v1.110.3 with Postgres, opencode 2.2.6, OpenShell 0.1.8, Docker 29.5.2): **yes, with three things zero would have to do and two findings against it.** The proxy part runs by `run.sh` (20 PASS, 0 FAIL, 1 NOTE). The sandbox part was run by hand once and is marked so.

## Chain

```
opencode in the sandbox ── LiteLLM key ──> LiteLLM (host, 127.0.0.1:4000) ──> GitHub Copilot (account connected in LiteLLM)
                                              └──────────────────────────> MCP server (token held by LiteLLM)
```

The stack is PoC 17's compose files plus `compose.yaml` here (a stand-in MCP server with two tools, `read_note` and `write_note`, that refuses calls without its own token). Everything LiteLLM does is declared in `config.yaml`: Copilot models by wildcard, the MCP server, two guardrails.

## Result

From `run.sh`:

| Check | Result |
|---|---|
| Model list of a key comes from LiteLLM's discovery | 68 Copilot models, nothing else |
| Copilot answers with the key | yes |
| Model outside the key's list | refused |
| No key | 401 |
| Key that does not belong to the LiteLLM user who connected Copilot | no Copilot |
| Key with tool list `read_note` | lists and calls `read_note` only; `write_note` refused |
| Key with both tools | lists and calls both |
| Key without tool list, no user or a normal user | no tool |
| Key without tool list, admin user | **both tools** (NOTE, see findings) |
| MCP server called without its token | 401; no client holds that token |
| Mail address in a prompt | reaches the model as `[EMAIL_REDACTED]` |
| Blocked word in a prompt / in a tool call | refused |
| Deleted key | refused |
| Master key or MCP token in the proxy log | not found |

By hand, in the sandbox `zero-developer` (not in `run.sh`):

| Check | Result |
|---|---|
| Proxy reachable from the sandbox | only after a policy rule for `host.openshell.internal:4000` |
| `opencode models` | 68 `litellm/...` models, from a config file rendered out of the key's `/v1/models` |
| `opencode run -m litellm/github_copilot/claude-haiku-4.5` | answered, after the key was given to opencode's service |
| MCP with curl from the sandbox | `read_note` only; `write_note` refused |
| MCP through opencode (`mcp.servers` entry pointing at the proxy) | connected; the model called `read_note`; no `write_note` exists for it |
| MCP server itself from the sandbox | not reachable |

## Findings

1. **opencode does not read the model list from the proxy.** A custom provider must list its models. zero would render `providers.litellm.models` from the key's `/v1/models` at `zero up`. Done here with `jq`.
2. **The key ends up readable in the sandbox.** opencode's service only sees it after `opencode service set env LITELLM_API_KEY ...`, which stores it in `service.json`. It is a LiteLLM key (limited models and tools, short life, deletable), not a provider credential. Keeping it outside as an OpenShell placeholder was not tried.
3. **Copilot is tied to the LiteLLM user who connected.** A key reaches Copilot only when it belongs to that user. Here that is the admin user.
4. **Keys of the admin user fail open for MCP.** With `require_key_mcp_access_defined: true` a key without a tool list gets no tool, unless it belongs to the admin user: then it gets all. Together with finding 3 this means: every sandbox key needs an explicit tool list, or Copilot is connected by a user who is not admin (not tried).
5. **Tool lists per key need the database.** Without Postgres only one list per server for everybody is possible.
6. Key per sandbox works through the API: `POST /key/generate` (alias, models, tools, lifetime), `POST /key/delete`.

## DSGVO (#129)

Only what was run. No claim beyond it.

| Data flow | Shown |
|---|---|
| Personal data to the model provider | partly: masked or refused when a pattern or word list finds it (mail address shown). Free text without a pattern is not found; that needs Presidio (own containers, not run; [docs](https://docs.litellm.ai/docs/proxy/guardrails/pii_masking_v2)) |
| Personal data to tools | partly: blocked word refused on a tool call; masking on tool calls not run |
| Personal data to the forge | not covered: git and forge traffic does not pass LiteLLM |
| Record of what was sent | not looked at |
| Where the data goes, contracts | not a technical control |

## Verdict per job

| Job | LiteLLM | Instead |
|---|---|---|
| Models | fits: one key, one list, provider login stays outside | provider login in the sandbox as today |
| MCP tool list | works, but needs Postgres, and identity is a key in the sandbox | agentgateway (ADR 0043, PoC 15): identity by SPIFFE, no database, list per request from zero-cp |
| Guardrails | fits for the built-in `litellm_content_filter` used here: declared in the config, no outside service ([docs](https://docs.litellm.ai/docs/proxy/guardrails/litellm_content_filter)). Other guardrails not looked at; by the docs Presidio needs its own containers and secret detection a licence | none in zero today |
| Skills, instructions | not run. By the docs: skills only from the database and only for Claude Code and Codex; no delivery of instructions ([docs](https://docs.litellm.ai/docs/skills_gateway)) | read-only folder from zero's config |

## Limits

- Sandbox part by hand, once. VS Code not run.
- Run with rootful Docker (C57). Tools only against the stand-in server.
- Not shown: "no provider token in the sandbox" and "model egress is the proxy only". The sandbox still held zero's own Copilot login and its egress rules; the LiteLLM path does not use them, dropping them was not tried.
- Blocking as it stands: Copilot answers only for keys of the connected LiteLLM user, here the admin, and a key of the admin user without a tool list sees every tool. A workspace key is safe only when it always carries an explicit tool list, or when Copilot is connected as a normal LiteLLM user (not tried).
- Weight as in PoC 17: 1.36 GB image, about 780 MiB with Postgres.

## Run

Needs PoC 17 run once with `UI=1` and a GitHub account connected in the LiteLLM UI.

```
COMPOSE="docker compose" pocs/18-litellm-sandbox/run.sh
```
