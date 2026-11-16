# PoC 12: OpenBao as credential broker backend (#98)

Question: can a GitLab web login give a user an OpenBao identity, and can a broker mint per-sandbox GitLab tokens from a root credential kept in OpenBao?

Result: yes. 15/15 PASS (OpenBao 2.7.2 dev mode, gitlab.com, 2026-11-16).

| Check | Result |
|---|---|
| GitLab web login yields an OpenBao token (OIDC auth, PKCE) | PASS |
| Role bound to another GitLab user is refused | PASS |
| User writes own path `secret/zero/users/<gitlab uid>/` | PASS |
| User denied on another user's path | PASS |
| GitLab OAuth login (code + PKCE, scope `api`) → refresh token stored in OpenBao | PASS |
| Refresh gives a 2h access token | PASS |
| Rotated refresh token stored back | PASS |
| Old refresh token is dead (`invalid_grant`) | PASS |
| Child token minted with the access token is fine-grained | PASS |
| Child reads the project | PASS |
| Child cannot read `/user` | PASS |
| Child works for git | PASS |
| Child revoked with the access token | PASS |
| Revoked child is refused (401) | PASS |
| Audit log names the GitLab user on each root credential access | PASS |

## Run

```
BAO=<bao binary> [PROJECT=<group/project>] pocs/12-openbao-broker/run.sh
```

Interactive: opens the browser twice (OpenBao OIDC login, GitLab OAuth authorize). Needs `glab` logged in, `jq`, `curl`, `git`, `python3`. Creates a throwaway user-level OAuth app and removes it, the child token and the OpenBao server at exit.

## Findings

- A user-level OAuth app can be created over the API without admin rights (`POST /user/applications`).
- Device authorization grant is refused for our own app: `POST /oauth/authorize_device` → `access_denied`, while the GitLab CLI app gets a device code. Cause unknown. Not worked around; authorization code + PKCE on a loopback redirect is used.
- GitLab rotates the refresh token on every use. The broker must be the single writer per user, otherwise the stored token goes stale.
- The OAuth access token has classic `api` scope (full user power). It exists only in the broker for up to 2h; the sandbox gets the fine-grained child.
- OpenBao 2.7 enables audit devices only through the config file, not through the API.
- OpenBao audit shows who accessed the root credential. Mint and revoke happen at GitLab and are not in the OpenBao audit log; the broker has to log them.

## Limits

- OpenBao in dev mode, no TLS, root token used for setup.
- OIDC role bound to one subject; a real setup binds by group claim.
- OAuth app secret is stored in the OpenBao OIDC config.
- Not wired into zero (`zero login`, `zero serve` backend, `zero install --openbao`: still open in #98).
