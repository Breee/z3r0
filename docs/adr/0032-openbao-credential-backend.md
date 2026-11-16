---
status: accepted
date: 2026-11-16
deciders: bree
requirements: [Z-INV-4, Z-INV-6]
---
# 0032 — OpenBao is the credential backend: opt-in storage now, brokering next

## Context
OpenShell keeps provider credentials AES-256-GCM encrypted in the gateway DB; the key file lies on the same disk (#87). PoC 10 showed the gateway's `vault` credential driver against OpenBao 2.7.2: the secret is absent from gateway state, present in OpenBao KV, and new credential use fails closed when OpenBao is down (6/6). The driver is static KV only; it cannot mint tokens.

## Decision
1. **Storage, opt-in.** Default stays the built-in encrypted store. A gateway may be pointed at an OpenBao server (`credential_drivers = ["vault"]`); `zero install --openbao <addr>` will set that up. Not default: an OpenBao on the same laptop disk protects nothing more than the key file does.
2. **OpenBao is the target credential backend** for everything beyond the laptop default. Dynamic tokens go through `zero serve`, not the gateway driver: `zero serve` logs in to OpenBao, reads the user's root credential, mints the per-sandbox fine-grained forge token, revokes it at session end.
3. **Root credential = the user's own forge grant** (GitLab OAuth refresh token, stored in OpenBao, never on the laptop in the central deployment). Agent tokens stay personal fine-grained tokens (ADR 0020).
4. **Login = GitLab web login.** `zero serve` / the user authenticate to OpenBao with a GitLab-issued identity (OpenBao JWT/OIDC auth). An enterprise IdP behind GitLab is covered by the same browser flow. SPIRE only in the cluster deployment.
5. Minting code is zero's own; the `ilijamt/vault-plugin-secrets-gitlab` plugin has no fine-grained tokens (`ai-docs/research-openbao-dynamic-tokens.md`).

## Consequences
- OpenBao holds refresh tokens with `api` scope: it and `zero serve` are the crown jewels (F3 in `docs/scenarios/identity-broker-usecases.md`). Accepted.
- One credential driver per gateway, no migration: switching means remove credentials, switch, recreate sandboxes.
- The gateway's own OpenBao token is a file on the gateway host (`token_file`; the only other method is kubernetes).
- Stopping OpenBao does not revoke: a running sandbox keeps its resolved secret (PoC 10).
- Not built or tested yet: `zero install --openbao`, `zero up` end to end on a vault-driver gateway, OpenBao login via gitlab.com, TLS/unseal/token renewal for a real OpenBao.
