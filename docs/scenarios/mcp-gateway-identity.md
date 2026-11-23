# MCP gateway with sandbox identity: GitLab and Kubernetes for the MVP

Status: draft 2026-11-20 (#127). First link proven in `pocs/13-agentgateway`; the rest is design. Decision input for ADR 0041.

Related: [kubernetes-access.md](kubernetes-access.md), [platform-engineer-usecases.md](platform-engineer-usecases.md), [identity-broker-usecases.md](identity-broker-usecases.md), [../attack-paths.md](../attack-paths.md).

## 1. Idea

The agent talks to GitLab and Kubernetes only through MCP tools behind one gateway. The gateway knows which sandbox calls, allows tools by policy, and holds the credentials. No API credential is in the sandbox, and the sandbox needs no network route to the cluster.

No part is written by us where a tested one exists:

| Job | Part | State |
|---|---|---|
| identity per sandbox | SPIRE (JWT-SVID per supervisor) | proven, PoC 08 |
| attach identity to requests, keep it from the agent | OpenShell supervisor (`token_grant` in a provider profile) | proven, PoC 09, 13 |
| decide which sandbox is a live session | zero-cp | proven as allow file, PoC 09, 13 |
| check identity, allow tools, log calls | agentgateway | proven with a test server, PoC 13 |
| credentials for GitLab | OpenBao broker (fine-grained token per sandbox) | proven, PoC 12 |
| credentials for Kubernetes | session ServiceAccount with read Roles | proven cluster side, #123 |
| tools | Kubernetes and GitLab MCP servers | not chosen, not tested |

## 2. Chain

```
agent ──▶ supervisor ──▶ agentgateway ──▶ MCP server ──▶ GitLab / Kubernetes API
             │  ▲            │                 ▲
   SVID      ▼  │ token      │ rules           │ narrow credential
           zero-cp ──────────┴─────────────────┘
             ▲
           SPIRE
```

1. `zero up` creates the sandbox and registers its SPIFFE ID in SPIRE and in zero-cp, with the capabilities the human approved.
2. The agent calls a tool. The supervisor gets a token for the gateway from zero-cp, using its SVID, and injects it.
3. agentgateway verifies the token and allows the tool only if a rule for this session names it.
4. The MCP server calls the API with a credential that can do no more than the capability: session ServiceAccount token, scoped GitLab token.
5. `zero rm` removes the registration, the rules and the credentials.

Two walls: the tool rule at the gateway and the narrow credential behind it. The credential is the real limit; the rule keeps the tool list small and gives the log.

## 3. What PoC 13 proved

15/15 checks, see `pocs/13-agentgateway/README.md`: a registered sandbox sees and calls only the tool its rule names; another sandbox with a valid identity gets nothing; no token, a forged token and a removed registration fail closed; the agent never sees the token and cannot replace it; the gateway log names the sandbox for every call.

## 3.1 What PoC 14 proved

25/25 checks, see `pocs/14-k8s-mcp/README.md`: one shared gateway serves two sandboxes and calls the Kubernetes MCP server with each session's own ServiceAccount token, which zero-cp hands out per request. Read works; Secrets, the other namespace, a capability not granted and writes fail; the cluster audit log names the right ServiceAccount; no token in the sandbox; a stopped session ends access at the next call.

## 3.2 What PoC 15 proved

32/32 checks, see `pocs/15-gitlab-mcp/README.md`: the shared gateway puts GitLab's own MCP endpoint (`https://<forge host>/api/v4/mcp`) behind it and gives each workspace its own tool list and its own GitLab token, both returned by zero-cp per request. A tool outside the list is hidden and refused; the other workspace's project is refused by GitLab; a removed tool or session ends access at the next request; no GitLab token in the sandbox. GitLab checks a separate token permission per tool.

## 4. Open decisions

| # | Question | Options |
|---|---|---|
| M1 | Which token does the gateway verify? | Decided (ADR 0043): a). Evidence: a) works in PoC 13 and 14; capabilities need not be claims, zero-cp answers per request. · a) the SVID itself (PoC 13: no issuer of our own, but no capability claims, rules per sandbox ID) · b) token from an identity provider that accepts SPIFFE client assertions (OpenBao, Keycloak; not tested) with session and capability claims · c) opaque key per session with agentgateway API-key auth |
| M2 | One gateway for all sessions or one per session? | Decided (ADR 0043): shared. Evidence: shared works, backend credential per request from zero-cp (PoC 14). · shared: one port, backend credential per session solved in PoC 14, tool rules per session not tested · per session: simple credential handling, more processes |
| M3 | Which MCP servers? | Decided for GitLab (ADR 0043): GitLab's instance endpoint, host from the workspace's forge (self-hosted too); a default tool list with an override per workspace. Evidence: PoC 15. Kubernetes open. · upstream Kubernetes and GitLab servers pinned by version (tool descriptions are input the agent reads) · tools written in zero-cp |
| M4 | Namespace and project limits | Open; by credential only until decided. · by credential only · also by rules on tool arguments (`mcpGuardrails`), one tested rule per tool |
| M5 | git traffic | Open; unchanged. · not MCP: push gate stays; fetch still uses a token in the sandbox today |
| M6 | Does the GitLab MCP move out of the sandbox too? | Decided (ADR 0043): yes, as an addition that can be switched off; tool list from zero-cp per request (variant B). Evidence: works in PoC 15, the token stays outside. · today zero configures it inside the sandbox with the token |

## 5. Flaws and gaps

- MG1: SPIRE, agentgateway, an identity provider and MCP servers are more parts to install and keep patched than zero has today (two binaries). `zero install` would have to set them up.
- MG2: The owner of the podman socket can get any sandbox's SVID (C49). On the laptop that is the human's own user or the gateway user.
- MG3: SPIRE entries are created per sandbox after creation (upstream demo is single-sandbox); zero has to register and remove them.
- MG4: Tool results are still input from outside (injection path, P26); the gateway can mask results but does not judge them.
- MG5: Read-only depends on the MCP server's tools; a tool that reads Secrets or runs exec must not be in any rule, and the credential must refuse it anyway.
- MG6: Streams (`logs -f`) through MCP: not tested.
- MG7: The upstream podman SPIFFE demo needs local changes to run on 0.1.8 (C49).
- MG8: SPIRE key rotation: gateway read the keys once from a file in the PoC.
- MG9: zero-cp is asked on every tool call (no cache); if it is down, all tools fail closed.
- MG10: The gateway proves itself to zero-cp with a shared key in a file.
- MG11: The SPIRE agent of the upstream demo stopped after about 2 hours (join-token attestation, agent SVID not renewed). A real setup needs a node attestation that survives.
- MG12: kubernetes-mcp-server needs `disable_localhost_protection` behind the gateway and binds all interfaces by default; it must be bound to loopback.
- MG13: Closed by PoC 15: tool rules per workspace work, with the list returned by zero-cp per request. The lists were files written by the script, not zero's config.
- MG14: GitLab checks token permissions per tool. Partly closed (#131): zero has a tool -> permission table for the 15 default tools (`gitlabTools` in `internal/zero/gitlab.go`), checked on gitlab.com one tool at a time, with and without its permissions (`pocs/15-gitlab-mcp/tools-check.sh`, output in `tools-check.out`). The token is minted from it: `tools` in the workspace config adds the listed tools' permissions to the token (#132, checked on gitlab.com: `list_branches` refused without the setting, works with it, refused again after removing it). Open: the list is not enforced at a gateway by zero yet. Not proven: `get_work_item`, and whether `list_work_items` needs `read_work_item` (test project has no work item; it is refused without `read_namespace`, and what else that permission shows is not known); `list_tags` is not refused with `read_repository_tag` but the project has no tag. Without `read_merge_request`, `list_merge_requests` returns an empty list instead of a refusal. `search_labels` returns the group's labels with `read_project` alone. A GitLab release can change the permission of a tool.
- MG15: The tool name of a call is only in the gateway log; zero-cp's decision log does not see it.
- MG16: GitLab's endpoint offers 15 write tools and can add tools with a release; only a list written by zero keeps them out.

## 6. Next proof steps

1. Done in PoC 14: Kubernetes MCP server behind the gateway with a session ServiceAccount.
2. Done in PoC 15: tool rules per workspace (MG13).
3. opencode in the sandbox as MCP client.
4. Done in PoC 15 with a token minted by glab: GitLab MCP endpoint behind the gateway. Open: token from the OpenBao broker, tool -> permission table (MG14), a self-hosted instance.
