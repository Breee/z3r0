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

## 4. Open decisions

| # | Question | Options |
|---|---|---|
| M1 | Which token does the gateway verify? | a) the SVID itself (PoC 13: no issuer of our own, but no capability claims, rules per sandbox ID) · b) token from an identity provider that accepts SPIFFE client assertions (OpenBao, Keycloak; not tested) with session and capability claims · c) opaque key per session with agentgateway API-key auth |
| M2 | One gateway for all sessions or one per session? | shared: one port, rules on claims, backend credential per session unsolved · per session: simple credential handling, more processes |
| M3 | Which MCP servers? | upstream Kubernetes and GitLab servers pinned by version (tool descriptions are input the agent reads) · tools written in zero-cp |
| M4 | Namespace and project limits | by credential only · also by rules on tool arguments (`mcpGuardrails`), one tested rule per tool |
| M5 | git traffic | not MCP: push gate stays; fetch still uses a token in the sandbox today |
| M6 | Does the GitLab MCP move out of the sandbox too? | today zero configures it inside the sandbox with the token |

## 5. Flaws and gaps

- MG1: SPIRE, agentgateway, an identity provider and MCP servers are more parts to install and keep patched than zero has today (two binaries). `zero install` would have to set them up.
- MG2: The owner of the podman socket can get any sandbox's SVID (C49). On the laptop that is the human's own user or the gateway user.
- MG3: SPIRE entries are created per sandbox after creation (upstream demo is single-sandbox); zero has to register and remove them.
- MG4: Tool results are still input from outside (injection path, P26); the gateway can mask results but does not judge them.
- MG5: Read-only depends on the MCP server's tools; a tool that reads Secrets or runs exec must not be in any rule, and the credential must refuse it anyway.
- MG6: Streams (`logs -f`) through MCP: not tested.
- MG7: The upstream podman SPIFFE demo needs local changes to run on 0.1.8 (C49).
- MG8: SPIRE key rotation: gateway read the keys once from a file in the PoC.

## 6. Next proof steps

1. Kubernetes MCP server behind the gateway with a session ServiceAccount on the kind cluster: read works, Secrets and writes fail at both walls.
2. Token with capability claims (M1 b) or per-session gateway (M2).
3. opencode in the sandbox as MCP client.
4. GitLab MCP server with the per-sandbox token from the OpenBao broker.
