---
status: accepted
date: 2026-11-16
deciders: bree
requirements: [Z-INV-3, Z-INV-4]
---

# 0038 — glab is the login; any GitLab host glab knows

## Context

ADR 0032 planned an own login (`zero login`) with the user's GitLab OAuth refresh token stored in OpenBao. PoC 12 (#98) showed it works, but it adds a second login, a second place holding a full-`api` credential, and an OAuth app to manage. Developers already use `glab`. GitLab is working on granular OAuth tokens (19.6, behind a disabled flag).

## Decision

- zero has no login of its own. The host's `glab` session mints, renews and revokes the per-sandbox fine-grained token (as since ADR 0020). No `zero auth`, no token storage in zero.
- The GitLab host comes from `origin`: gitlab.com, or any host `glab` is logged in to (`glab auth status --hostname <host>`). The host is recorded in state (`forge.host`); every `glab` call names it.
- One OpenShell profile per host: `zero-gitlab` for gitlab.com, `zero-gitlab-<host>` otherwise.
- No glab login for the host: `zero up` stops and prints `glab auth login --hostname <host> --use-keyring`.
- `zero doctor` warns when glab keeps the host's token in plain text in its config file.
- OpenBao stays an opt-in credential store for the gateway (ADR 0032 point 1). Points 2–5 of ADR 0032 (brokering through OpenBao, refresh token in OpenBao, own login) are superseded.

## Consequences

- One login the developer already has; renewal by `zero serve` and revoke on `rm`/`pause` keep working unattended.
- The glab credential (full `api` scope, refresh token) stays on the laptop, in plain text unless the keyring is used. When granular OAuth tokens reach GitLab, the glab login itself can be narrow.
- Revocation still depends on the laptop (Z-INV-6 open until a central deployment, #92).
- A host glab is not logged in to is not recognised as GitLab: `zero up` runs without forge and `zero doctor` says so.
- Not tested: a self-hosted GitLab instance (only gitlab.com), including fine-grained tokens, MCP endpoint and push options there.
