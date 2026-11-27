# PoC 27 — MCP tools on the model proxy's key: servers in the config, list changed on a living key, GitLab's endpoint behind it (#164)

Question: can zero declare MCP servers in the LiteLLM config it writes, give each sandbox key its tool list, change that list without a new key, and put GitLab's own MCP endpoint behind the proxy with a token that stays there?

Result (2026-11-27, LiteLLM v1.110.3 with Postgres, Docker 29.5.2, gitlab.com): **yes**, 15 PASS, 1 NOTE. Two things the build has to solve: the upstream token is one per server entry and read when the proxy starts, and the proxy keeps no record per tool call that names key and tool.

## Sources

- <https://docs.litellm.ai/docs/mcp_config_reference> (`mcp_servers` in the config, `auth_type`, `auth_value: os.environ/...`, client endpoint `/mcp`, both key headers)
- <https://docs.litellm.ai/docs/mcp_control>, <https://docs.litellm.ai/docs/mcp_grant_access> (`object_permission`, `no-mcp-servers`, `POST /key/update`)
- <https://docs.litellm.ai/docs/mcp_troubleshoot> (methods, `Accept` header), <https://docs.litellm.ai/docs/mcp_cost> (hook after each tool call)

## Chain

```
client --key--> LiteLLM /mcp --token from its environment--> stand-in server "notes"
                             \--token from its environment--> https://gitlab.com/api/v4/mcp
```

## Result

| Check | Result |
|---|---|
| Key with a tool list sees that tool only; a tool outside it is refused | PASS |
| Key with `no-mcp-servers` sees no tool | PASS |
| `/mcp` accepts the key in the `Authorization` header (the form the sandbox placeholder uses) | PASS |
| `POST /key/update` with a new list: the next request lists and calls the added tool | PASS |
| List taken away with `/key/update`: the next request lists nothing, the call is refused | PASS |
| GitLab's endpoint behind the proxy: key limited to `get_project` lists `gitlab-get_project` only | PASS |
| `gitlab-get_project` answers through the proxy; no GitLab token in the answer | PASS |
| Another GitLab tool is refused by the proxy; a key without the server sees none of its tools | PASS |
| No key or token in the proxy log | PASS |
| Spend log rows for the tool calls | NOTE: none |

## Findings

1. MCP servers go into the config file; the lists per key go through the API and need the database. Tool names reach the client as `<server>-<tool>`.
2. A key's list can be changed while the key lives; the change holds at the next request. zero does not need a new key when a workspace's tools change.
3. GitLab's instance endpoint works behind LiteLLM as it is.
4. The upstream token is one per server entry, read from the proxy's environment at start. Tokens per workspace mean one entry per workspace, and a new token means the proxy is started again. Not run.
5. The proxy's log shows `POST /mcp` and, for a refused call, the tool name; it names no key. The spend log holds no row per tool call. A record with key and tool needs the documented hook (`async_post_mcp_tool_call_hook`). Not run.

## Limits

- Not run from a sandbox: zero's rule for the proxy does not open `/mcp` yet. opencode as the client was shown in PoC 18.
- One GitLab project, one token with `read_project`.
- Docker only (ADR 0051). `run.sh` starts and stops the PoC 17/18 stack.

## Run

```
pocs/27-mcp-on-proxy-key/run.sh
```

Needs the state of PoC 17 (`~/.local/state/zero-poc17`), `glab` logged in to the forge, `jq`. The GitLab token it makes is revoked at the end.
