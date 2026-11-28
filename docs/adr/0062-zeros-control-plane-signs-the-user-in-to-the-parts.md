---
status: proposed
date: 2026-11-28
deciders: bree
requirements: [Z-INV-1]
---

# 0062 — zero's control plane signs the user in to the parts

## Context

The model proxy (LiteLLM) has its own page with its own typed login (user
`admin`, password in a file). The zero UI cannot sign the user in there
(C59): the front-proxy way needs LiteLLM Enterprise. The user decided that
one sign-in comes with an identity provider (#179).

PoC 36 showed: LiteLLM v1.110.3 accepts a small OpenID provider with three
addresses (authorize, token, user info). The browser lands in LiteLLM's
dashboard with nothing typed, the user is proxy admin, and the typed login
can be turned off. It is free for up to 5 users.

## Decision

1. zero's control plane (zero-cp) is the OpenID provider for the local,
   single-user case. No extra container.
2. It approves a sign-in only for a browser that holds a valid zero UI
   session. Without one it sends the browser to the zero UI's entry.
3. One client per part, with a secret zero makes and keeps in its state
   directory, and one fixed redirect address. A code works once and for a
   short time.
4. The provider names one user, with no email (a reserved-name address
   breaks LiteLLM's callback).
5. LiteLLM gets the provider's addresses, that user as `PROXY_ADMIN_ID`,
   the automatic redirect, and typed login off.
6. Teams use an existing provider (Dex or Keycloak) in place of zero-cp's.
   That is its own decision, later.
7. C59 closes once this is built.

## Consequences

- One sign-in: the zero UI's link opens LiteLLM's page too.
- The admin password file is no longer the way in. The master key stays
  for zero's own calls.
- LiteLLM still puts a UI key into a cookie that page scripts can read.
  zero cannot change that. It is stated in the guide.
- zero-cp gains three endpoints that must be right: redirect check, client
  secret check, one-time codes.
- More than 5 users needs LiteLLM Enterprise or another way.
- Not shown yet: the provider inside zero-cp, the sign-in behind the zero
  UI's `/litellm/` path, the installed proxy.
