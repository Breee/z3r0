---
status: accepted
date: 2026-11-27
deciders: bree
requirements: [Z-INV-1, Z-INV-2, Z-INV-4, Z-INV-6]
---

# 0055 — A workspace's tools go through the model proxy when it is on

## Context

A workspace can list GitLab MCP tools (`tools:`, #132). Until now the list only widened the sandbox's own token, and since ADR 0046 the sandbox cannot reach GitLab's MCP endpoint, so the agent had no tool. ADR 0049 (amended) put tool lists on the sandbox's model proxy key. PoC 27 showed it: MCP servers declared in the proxy's config, the list on the key, GitLab's endpoint behind the proxy, and that the proxy reads one upstream token per server entry from its environment at start (#164).

## Decision

1. In a workspace with `models` (model proxy on) and `tools`, the tools are reached through the proxy. Without the proxy nothing changes: the list widens the sandbox's token and no tool is reachable.
2. Each such workspace gets its own server entry in the proxy's config, `gitlab_<workspace>` (a `-` in the name becomes `_`; LiteLLM refuses a `-`), pointing at `https://<forge host>/api/v4/mcp`.
3. Each entry has its own GitLab token, made by zero for the workspace's projects with only what its tools need (plus `read_project` and `execute_mcp_tool`); it cannot push. The token is kept in zero's state and in the proxy's environment (`parts/mcp.env`, readable by the user only). It is never in the sandbox. The sandbox's own token then carries no tool permissions.
4. The tool list is on the sandbox's proxy key; a key without tools carries `no-mcp-servers`. The sandbox reaches `POST /mcp` of the proxy and nothing else new.
5. opencode in the sandbox gets one remote MCP server, `gitlab`, pointing at the proxy with the key's placeholder.
6. The token is made or renewed at `zero up` (when the list changed or less than 12 hours are left) and revoked at `zero pause` and `zero rm`. When entries or a token change, the proxy is made anew.

## Consequences

- Making the proxy anew takes about 21 s (PoC 27); every sandbox that uses the proxy waits that long.
- A tool token is not renewed while the sandbox runs. After two days the tools stop until the next `zero up`.
- Tools are named `gitlab_<workspace>-<tool>`.
- No record per tool call that names workspace and tool: LiteLLM's spend log holds none, its callback for this was not tried. The proxy's log names a refused tool, not the key.
- GitLab only. Copilot CLI and Claude Code are not told about the server.
- Without the model proxy the agent has no MCP tool. The agentgateway way (ADR 0043) is not built (#178).
- Amends ADR 0043 and ADR 0049.
