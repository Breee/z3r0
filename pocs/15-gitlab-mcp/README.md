# PoC 15 — GitLab's instance MCP endpoint behind the shared agentgateway, tool list per workspace (#130)

Question: can the shared gateway give each workspace its own tool list and its own GitLab token for GitLab's MCP endpoint, with no GitLab token in the sandbox?

Result (2026-11-23, OpenShell 0.1.8, SPIRE 1.12.4, agentgateway 1.7.0, gitlab.com MCP server 19.7.0-pre, podman): **yes**. `run.sh` 28/28 PASS.

## Chain

```
agent (curl) -> supervisor injects JWT-SVID -> agentgateway :18091
   jwtAuth: verifies the SVID with SPIRE's keys
   extAuthz: asks zero-cp (subject = SPIFFE ID, gateway key) -> workspace's GitLab token + tool list
   mcpAuthorization: mcp.tool.name in extauthz.tools.split(",")
   backendAuth: sets the token for this request
-> https://<forge host>/api/v4/mcp (GitLab's own endpoint, no MCP server of ours)
```

The endpoint is built from the workspace's forge host (`FORGE_HOST`, default gitlab.com), so a self-hosted instance takes the same path. Only gitlab.com was run.

Workspaces: a = token for project A, tool list overridden to `get_project list_branches`; b = token for project B, default list (`default-tools.txt`, 15 read tools). Both tokens have zero's `git.read` permissions plus `execute_mcp_tool`.

## Result

| Check | Result |
|---|---|
| a lists tools | exactly its override (2 tools) |
| b lists tools through the same gateway | exactly the default list (15 tools) |
| a, b call `get_project` on their own project | works |
| a calls a tool outside its list (`list_commits`), a write tool (`save_note`) | refused by the gateway |
| a reads b's project, b reads a's project | refused by GitLab: the token is the other workspace's |
| b calls a tool in its list that its token does not cover (`list_branches`) | refused by GitLab (`insufficient_granular_scope`) |
| call without identity, call with a GitLab token as identity | 401 |
| subject header sent to zero-cp without the gateway key | 403 |
| GitLab token in the sandbox (env, files, responses, tool results), in the gateway log | none |
| GitLab reached from the sandbox directly | no |
| tool removed from a's list | gone from the list and refused at the next request |
| workspace without a tool list | no tool listed, call refused |
| a's session removed in zero-cp | gateway fails closed (`no session`), b still works |
| variant A: one static rule per workspace in the gateway config | works; a changed rule is picked up without restart |

## Variants

- **A: zero writes one rule per workspace into the gateway config** (`jwt.sub == "<id>" && mcp.tool.name in [...]`). Works, and agentgateway reloads the file without restart. zero would have to rewrite a shared file on every workspace start, stop and change.
- **B: zero-cp returns the list with each request**; the gateway config holds one fixed rule. Denied tools are hidden from `tools/list` and refused on call. A change takes effect at the next request. The gateway config never changes.

Recommendation: B.

## Findings

- GitLab checks one fine-grained token permission per tool. With `git.read`: `get_project` and `list_commits` work; `list_branches`, `list_repository_tree` and `list_pipelines` are refused; `list_merge_requests` returns an empty list instead of a refusal. So the tool list and the token's permissions are two separate layers, and a tool in the list is only useful if the token covers it. zero needs a tool -> permission table; not built.
- GitLab's endpoint keeps no session; the gateway issues the MCP session ID.
- The endpoint offers 51 tools, 15 of them writes. The default list is written out by hand; it does not follow the server's `readOnlyHint`, which the server could change.
- zero-cp does not see the tool name (the authorization request carries only the subject). The tool name per call is in the gateway log only: `jwt.sub=<SPIFFE ID> mcp.method.name=tools/call gen_ai.tool.name=<tool>`, denials with `error="mcp: authorization denied for tool: <tool>"`.

## Invariants

| Invariant | Check | Gap |
|---|---|---|
| Z-INV-1 checks outside the sandbox | SVID check, tool rule and token run in the gateway and zero-cp; the sandbox cannot reach GitLab directly | — |
| Z-INV-2 no ambient authority | no list -> no tool; token limited to one project and `git.read` | tool -> permission table missing, so the token is not derived from the tool list |
| Z-INV-3 authenticated, attributable | no or wrong identity -> 401; gateway log names workspace and tool per call | two logs (gateway: tool, zero-cp: decision), not joined |
| Z-INV-4 no long-lived credential in the sandbox | no `glpat-` in env, files, responses | the token itself lives 1 day and sits in a file next to zero-cp |
| Z-INV-5 containment after supply-chain compromise | the MCP server is GitLab itself, outside the sandbox; it only ever sees the workspace's own token | tool descriptions and results are input the agent reads (MG4); the gateway image is pinned by digest but not verified by signature |
| Z-INV-6 revocation | tool removed or session removed -> refused at the next request | the GitLab token is revoked by the script at the end, not at session removal |

## Core and addition

The MCP gateway is an addition. Core (sandbox, workspaces, push gate, glab login) was not changed and does not need it. A workspace without a tool list gets no tool (checked). Switching the addition off for a workspace (provider not attached, zero's sandbox-side MCP config as today) was not run as a check.

Seam, three points:

1. sandbox identity: the SPIFFE ID the supervisor injects for the gateway's host;
2. workspace config: `tools` (absent = default list or off, to be decided) and the forge host;
3. session start/stop: zero-cp gets or drops the workspace's token and tool list.

## Portability

Run as containers: SPIRE, OpenShell gateway, agentgateway (`ghcr.io/agentgateway/agentgateway@sha256:460dda87…`), zero-cp (built in `golang@sha256:f496f8c5…`, runs from scratch).

Exceptions:

- host binaries `podman`, `openshell`, and `glab` (mints the tokens, as zero does today), `jq`, `curl` in the script;
- the zero-cp image is built locally, so it has no pinned digest (its base has);
- both containers use the host network;
- `ghcr.io/agentgateway` had to be added to the podman trust policy (accept without signature).

## Limits

- Client is curl, not opencode.
- Only gitlab.com was run; a self-hosted instance was not.
- agentgateway listens on all interfaces; the gateway key is a shared secret in a file (MG10).
- zero-cp is the PoC from `pocs/09-zero-cp` with a new `-tools` flag; tool lists are files written by the script, not zero's config.
- Write tools were only tested as refused, never allowed.

## Run

Needs the PoC 08 environment (SPIRE + gateway `spiffe-poc`) and `glab` logged in to the forge host.

```
IMAGE=<image with curl> OPENSHELL_SRC=<OpenShell checkout with the C49 changes> \
PROJECT_A=<group/project> PROJECT_B=<group/project> [FORGE_HOST=gitlab.example.org] \
pocs/15-gitlab-mcp/run.sh
```
