---
status: accepted
date: 2026-11-24
deciders: bree
requirements: [Z-INV-2, Z-INV-3, Z-INV-4]
---
# 0047 — The sandbox token stays a fine-grained token of the developer; no service account for now

## Context

zero mints one fine-grained personal access token per sandbox with the developer's glab login (ADR 0020, ADR 0038): limited to the workspace's projects and to the permissions its capabilities and tools need, valid until two days from now, revoked at pause and remove. #91 asked for a tested comparison with a named service account (`zero-<human>`, Developer role, tokens minted by OpenBao) so that both could be offered.

## Decision

1. The fine-grained token of the developer stays the only token type of zero.
2. The service account mode is not tested and not built now. The comparison of #91 was not run; nothing in this ADR is a test result about service accounts.
3. The question comes back when GitLab offers fine-grained OAuth tokens (expected by the developer, not checked here), because that changes what the login can hand out, or when a team deployment needs tokens minted without the developer's laptop (§28.5, #92).

## Consequences

- What GitLab records as the actor of an agent action is the developer. The session is told apart by the token name (`zero-<workspace>@<host>`) and the push gate log.
- The token can never do more than the developer may do in the project, and less wherever the fine-grained permissions cut it. A role below the developer's own (e.g. Developer for a Maintainer) cannot be set this way.
- Tokens are minted on the laptop; revocation without the laptop needs the forge's web page.
- One mode to document, test and attack (`tests/attacks/forge.sh`).
