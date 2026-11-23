# Tool eval: Warden (2026-11-23)

Question: can zero use Warden, and is there a benefit? Verdict first, evidence after.
Docs only: README, the whole docs site and the repository at v0.22.0 (2026-11-06), read in two passes on 2026-11-23. Nothing run. The evaluation with runs is #133.

Source: https://github.com/stephnangue/warden, https://wardengateway.com

## Verdict

| Tool | Layer | Fit | Now? |
|---|---|---|---|
| Warden | beside zero: access broker for every upstream (GitLab API, git over HTTPS, Kubernetes, LLM APIs, OpenBao, clouds, MCP). Verifies the agent's identity, decides per request, injects the upstream credential, audits | could take over zero's token minting, the planned OpenBao broker, agentgateway + zero-cp (PoC 15) and the LLM key injection; fits a company setup with many users, too heavy for one user | no; the local version keeps its own core and ADR 0043 stays. Candidate layer for an enterprise setup, evaluated in #133 |

## What it would add

- MCP policy per identity: tools hidden from the list and refused on call, plus conditions on call arguments (agentgateway + zero-cp: tool name only).
- JWT-SVID verified against a bundle file or a federation URL, no SPIRE socket needed. SPIFFE ID mapped to a role by exact string or glob.
- Upstream credential chosen per request by role; read from OpenBao per request is supported. GitLab provider mints project or group access tokens, self-hosted instances supported.
- One audit entry per call with identity, tool, decision and matched rule. Would close MG15 (`docs/scenarios/mcp-gateway-identity.md`).
- Acting on behalf of a person and an agent registry with a sponsoring team (not needed by zero today, relevant for M4).

## Access brokering beyond MCP

Every backend works the same way: the agent calls Warden (`/v1/<mount>/role/<role>/gateway/<upstream path>`, or headers `X-Warden-Provider`, `X-Warden-Role`) with its JWT-SVID on each request, no login and no Warden token. Clients must address Warden; there is no forward proxy and no TLS interception.

| zero need | Warden | Fit |
|---|---|---|
| Sandbox identity by JWT-SVID | `spiffe` auth, presented per request, role decided per request | yes |
| GitLab API, least privilege per workspace | mints a project or group access token per role, deletes it at lease end; path and method policy; self-hosted via `gitlab_address` | yes, classic scopes only |
| git clone and push over HTTPS | smart HTTP proxied for GitLab and GitHub | yes; no check of branch or pack content |
| Kubernetes API per session | ServiceAccount token by TokenRequest, default 1 h | partly; token not revocable, `kubectl` through the path prefix not documented |
| LLM key kept out of the sandbox | Anthropic, OpenAI, Mistral, Cohere; model allow list by a condition on the body; streaming | yes |
| OpenBao as secret store | credential source and backend | yes |
| Audit per call with identity | JSON lines, request and response entry | yes; file only |
| Revocation at the next request | suspending an agent or editing a role applies at the next request | partly; no command to revoke one issued credential, admission cached up to 1 h |
| Acting for a named person | second principal, token exchange (RFC 8693) | yes |
| Several teams | namespaces, agent registry with sponsoring team | yes |
| Human approval for risky actions | none, no UI | no |
| Config from files | server config only | partly |
| git over SSH, container registries | none | no |

Policy sees method, path and JSON body fields, not headers, query string or git pack content. A push to a protected branch cannot be judged, so zero's push gate stays. No rate limits, quotas or LLM cost accounting, no metrics or tracing (upstream roadmap, issue 173). `glab` through Warden is not documented.

## Misfits for zero

- Needs Postgres, initialisation and unseal. No file storage. Against "one user, one host, containers only".
- Roles, groups and policies only through API or CLI, no declarative files. zero would have to push its workspace config into Warden and keep it in sync.
- GitLab fine-grained tokens not found in the docs, only classic scopes. zero's tool -> permission table (#131, #132) relies on them.
- No external authorization hook (no webhook, OPA, ext_authz). zero-cp could not stay the place that decides; the decision would live in Warden's policies.
- Maturity: one human author (811 of 858 commits), first release 2026-02-22, v0.22.0 with 12 breaking changes, mixed versions in a cluster not supported.
- Supply chain: images not signed, no SBOM, `SECURITY.md` empty (no disclosure process).
- Stated upstream limits: batched list calls are denied; no CRL/OCSP for X.509 SVIDs; argument conditions are not evaluated when the tool list is filtered.

## Collaboration

Things we would ask for upstream if #133 confirms them: file storage or declarative roles and policies, fine-grained GitLab tokens, signed images with SBOM, a security policy. Drafts go to `ai-docs/upstream/` when the evaluation runs; none written yet.
