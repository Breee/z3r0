# PoC 36: one sign-in for LiteLLM's page through a small OpenID provider

Issue #179. Caveat C59.

## Question

Can LiteLLM's own page open without its typed login, when a small OpenID
provider that zero controls signs the user in?

## Result (2026-11-28, LiteLLM v1.110.3)

Yes. 11 PASS, `fails: 0`. A fresh browser that opens `/litellm/ui/` lands in
the dashboard with no password field and no click.

| Check | Result |
|---|---|
| proxy starts with the OpenID settings | PASS |
| sign-in ends in the UI, nothing typed | PASS |
| no key in the address | PASS |
| session cookie set | PASS |
| signed in as the provider's user, as admin | PASS |
| the session's key works on an admin call | PASS |
| typed login refused | PASS |
| provider: wrong client secret refused | PASS |
| provider: a code works once, not twice | PASS |
| provider: another redirect address refused | PASS |
| browser: dashboard without a login page | seen by hand |

## Findings

- The provider needs three addresses only: authorize, token, user info.
  LiteLLM asks for no discovery document and no signing keys.
- LiteLLM sends its client secret with HTTP Basic to the token address.
- `PROXY_ADMIN_ID` set to the provider's user id makes that user the proxy
  admin.
- `AUTO_REDIRECT_UI_LOGIN_TO_SSO=true` is honoured by this image: no login
  page with a button comes first.
- `disable_password_login_when_sso_enabled: true` turns the typed login off
  (403).
- After the sign-in LiteLLM sets a cookie `token`: a JWT signed with the
  master key that holds a UI key. Page scripts can read it. So "no admin key
  in the browser's storage" (GOOD of #179) is not met by LiteLLM itself.
- A user email under a reserved name (`local@zero.invalid`) makes the
  callback answer 500. Sending no email works.
- The sign-in is free for up to 5 users (BerriAI/litellm#31734: the count
  includes users made another way).

## Sources

- https://docs.litellm.ai/docs/proxy/admin_ui_sso.md
- `litellm/proxy/management_endpoints/ui_sso.py`, tag v1.110.3

## Limits

- `idp.py` is a test script. It approves everyone who reaches it. The real
  provider must approve only a valid zero UI session.
- Run against its own throwaway proxy, not the installed one, and not behind
  zero's UI path on port 18097.
- One user. Teams and an existing provider (Dex, Keycloak) not tried.

## Run

```sh
pocs/36-one-sign-in/run.sh          # ends with fails: 0
HOLD=1 pocs/36-one-sign-in/run.sh   # keeps it up for a browser, enter ends
```

Needs Docker (default context) and ports 18796, 18797.
