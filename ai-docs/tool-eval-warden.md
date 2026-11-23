# Tool eval: Warden (2026-11-23)

Question: can zero use Warden, and is there a benefit? Verdict first, evidence after.
First look only: README, docs site and repository at v0.22.0 (2026-11-06). Nothing run. The evaluation with runs is #133.

Source: https://github.com/stephnangue/warden, https://wardengateway.com

## Verdict

| Tool | Layer | Fit | Now? |
|---|---|---|---|
| Warden | beside zero: gateway that verifies the agent's identity, decides per request, injects the upstream credential, audits | does the job of agentgateway + zero-cp (PoC 15) in one service; fits a company setup with many users, too heavy for one user | no; ADR 0043 stays. Candidate for an enterprise setup, evaluated in #133 |

## What it would add

- MCP policy per identity: tools hidden from the list and refused on call, plus conditions on call arguments (agentgateway + zero-cp: tool name only).
- JWT-SVID verified against a bundle file or a federation URL, no SPIRE socket needed. SPIFFE ID mapped to a role by exact string or glob.
- Upstream credential chosen per request by role; read from OpenBao per request is supported. GitLab provider mints project or group access tokens, self-hosted instances supported.
- One audit entry per call with identity, tool, decision and matched rule. Would close MG15 (`docs/scenarios/mcp-gateway-identity.md`).
- Acting on behalf of a person and an agent registry with a sponsoring team (not needed by zero today, relevant for M4).

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
