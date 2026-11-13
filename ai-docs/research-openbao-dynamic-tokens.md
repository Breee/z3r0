# Research: OpenBao minting short-lived forge tokens (2026-11-13, #88)

Question: can OpenBao replace zero's own `glab` token minting with short-lived, centrally revocable tokens?

## Findings

| Topic | Result | Source |
|---|---|---|
| GitLab plugin | `ilijamt/vault-plugin-secrets-gitlab` v0.12.1, MIT, active, states Vault/OpenBao support, published as OpenBao OCI plugin (`plugin "secret" "gitlab"` block, sha256-pinned) | repo README, `docs/openbao-oci.md` |
| Token types on gitlab.com | personal + user-service-account: **not supported** on gitlab.com. Supported: project/group access tokens, **group/project service account** tokens, deploy tokens, trigger tokens | README "GitLab Support" |
| Project access tokens | gitlab.com needs Premium/Ultimate | GitLab `project_access_tokens.md` |
| Service accounts | Free tier since GitLab 18.11, 100 per top-level group on gitlab.com; project service accounts since 18.10 (min 18.11 in plugin). Plugin does not create the account, only its tokens | GitLab `service_accounts.md`, plugin README |
| Fine-grained tokens | Plugin has no fine-grained (granular) support: PRs #333/#334 closed unmerged. Tokens use classic scopes (`api`, `read_repository`, `write_repository`, …) | plugin PR list |
| TTL | min 1h (plugin revokes on lease end), 24h if GitLab expires it; GitLab expiry is day-granular | plugin `docs/roles.md` |
| Plugin's own credential | needs a GitLab token with rights to create tokens (stored in OpenBao, can auto-rotate) | plugin README "Security model" |
| GitHub plugin | `martinbaillie/vault-plugin-secrets-github` v2.4.0 (Apache-2.0): GitHub App installation tokens, 1h, per-repo + per-permission. OpenBao SDK port: `rknightion/openbao-plugin-secrets-github` (3 stars, young) | repo READMEs |
| OpenBao SPIFFE auth | no native SPIFFE auth (openbao#3507 open). JWT auth method exists (`bound_subject`, `bound_audiences`, `bound_claims` with globs) | openbao issue, `docs/auth/jwt` |
| OpenShell → OpenBao directly | not possible: vault credential driver is static KV; `token_grant` expects an OAuth token endpoint, OpenBao login/secret responses are not OAuth-shaped | PoC 10, OpenShell 0.1.8 docs |

## What this means

1. Path is: supervisor JWT-SVID → zero-cp (PoC 09) → zero-cp logs in to OpenBao with its own JWT-SVID (JWT auth) → reads `gitlab/token/<role>` → returns token to supervisor. OpenBao revokes at lease end, independent of the laptop (Z6).
2. Agent identity becomes a **GitLab service account** with Developer role on the project, not the developer's own user. This removes "token inherits Maintainer" (ADR 0020, P20) — a bigger gain than the short lifetime.
3. Loss: classic scopes instead of fine-grained permissions (`write_repository` + `api` is broader per token, but the account only reaches projects it is a member of). `.zero/policy.yaml` capabilities can then only pick scopes, not single permissions.
4. Commits/MRs are attributed to the service account, not the developer — accountability must come from zero-cp's decision log (who started which sandbox).
5. Someone must create the service account and add it to the project (group Owner/Maintainer action, once per project or group).

## Not verified

- Plugin running on OpenBao 2.7.2 (OCI load needs a container runtime on the OpenBao host).
- Service account creation on the `corewire` group on gitlab.com Free (not tried: write action in a shared group).
- OpenBao JWT auth against SPIRE's JWKS (`jwks_url` / OIDC discovery provider).
- Whether GitLab MCP (`execute_mcp_tool`, user-boundary permission) works with a service-account classic token.
- GitHub side end-to-end.
