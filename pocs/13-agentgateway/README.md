# PoC 13 — agentgateway serves MCP tools to a sandbox identity (#127)

Question: can an agent reach tools only through an MCP gateway that knows which sandbox is calling, without a credential in the sandbox?

Result (2026-11-20, OpenShell 0.1.8, SPIRE 1.12.4, agentgateway 1.7.0, podman): **yes**. `run.sh` 15/15 PASS.

| Check | Result |
|---|---|
| sandbox a (named in the gateway rule) lists tools | only `echo` |
| sandbox a calls `echo` | works |
| sandbox a calls `get-sum` (not in the rule) | error, tool not run |
| sandbox b (valid identity, no rule) lists tools, calls `echo` | no tool, call denied |
| call without token (host process) | 401 |
| forged token | 401 |
| agent sends its own `Authorization` header | replaced by the supervisor |
| agent looks for the token (env, files, response) | not found |
| registration removed in zero-cp | next request fails closed |
| gateway log | every MCP request carries `jwt.sub=spiffe://…/sandbox/<id>`, method, target |

## How it works

```
agent (curl) ──▶ supervisor ──▶ agentgateway :18091 ──▶ MCP server (stdio)
                    │  ▲
       JWT-SVID     ▼  │ token
                  zero-cp :18090        SPIRE (identity per sandbox)
```

1. SPIRE gives each sandbox supervisor a JWT-SVID `spiffe://openshell.local/openshell/sandbox/<id>` (PoC 08).
2. `profile.yaml`: for requests to the gateway the supervisor asks zero-cp for a token (`token_grant`, `jwt-spiffe` assertion, audience `zero-mcp`) and injects it. The agent has no part in this.
3. zero-cp (PoC 09 binary, new flag `-svid`) verifies the SVID, checks the allow file and returns the SVID itself as the token. It holds no signing key.
4. `gateway.yaml`: agentgateway verifies the token against SPIRE's JWT keys (`jwtAuth`, strict) and allows tools by rule (`mcpAuthorization`): `mcp.tool.name == "echo" && jwt.sub == "<sandbox a>"`.
5. The MCP server is `@modelcontextprotocol/server-everything` (test server), started by agentgateway over stdio.

## Reproduce

Start SPIRE + throwaway gateway as in `pocs/08-spire-podman/README.md`, then:

```
OPENSHELL_SRC=<OpenShell v0.1.8 clone> AGENTGATEWAY=<agentgateway v1.7.0 binary> pocs/13-agentgateway/run.sh
```

`IMAGE=<image with curl>` overrides the sandbox image (the image trust policy of the podman user must allow it).

## Findings

- SPIRE's bundle (`bundle show -format spiffe`) is not accepted as JWKS by agentgateway: `the key is missing the kid attribute` (the X.509 key has none). `run.sh` passes only the `jwt-svid` keys.
- agentgateway filters `tools/list` by the same rule, so a sandbox does not see tools it may not call.
- The supervisor overwrites an `Authorization` header set by the agent.

## Limits / not covered

- The token is the SVID itself: same audience for zero-cp and the gateway, 5 minutes lifetime, no claims for capabilities. Rules name sandbox IDs; a real setup needs a token with session claims or one rule set per session.
- JWT keys are a file read at start; SPIRE key rotation is not handled (agentgateway can read a JWKS URL; not tested with the SPIRE OIDC discovery provider).
- agentgateway listened on all interfaces (`*:18091`) and plain HTTP; its admin UI on `127.0.0.1:15000`. Not hardened.
- Test MCP server only: no Kubernetes or GitLab server, no backend credential, no rule on tool arguments (`mcpGuardrails`).
- Only `curl` as client; opencode as MCP client not tested.
- zero-cp and agentgateway ran as host processes.
- Same trust boundary as PoC 08: the podman socket owner can obtain any sandbox's SVID (C49).
