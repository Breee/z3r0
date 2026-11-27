# LiteLLM: sign-in to the Admin UI from a front proxy without an Enterprise licence

**Target:** new feature request at https://github.com/BerriAI/litellm/issues (not filed; no existing issue found with a search for `custom_ui_sso_sign_in_handler` on 2026-11-27). Caveat C59, zero issue #163.

## Use case

A local tool (zero) starts LiteLLM for one person on their own machine, bound to 127.0.0.1, and already holds the master key to mint keys. It has its own local web UI with its own session. The person should reach LiteLLM's Admin UI from there without typing the master key again.

## What the docs offer (v1.110.3)

- `general_settings.custom_ui_sso_sign_in_handler` with `general_settings.trusted_proxy_ranges`: a front proxy sets identity headers, LiteLLM signs the browser in. https://docs.litellm.ai/docs/proxy/custom_sso says: "The custom UI sign-in hook for an upstream authentication proxy requires Enterprise."
- `/login`, `/v2/login`: private endpoints, no stability promise (https://docs.litellm.ai/docs/api_stability_policy).
- No documented link that carries a one-time token.

## Request

One of:

1. The header sign-in from a trusted proxy range available without a licence for a single admin user (the same bound as "SSO is free for up to 5 users").
2. A documented call that turns the master key into a one-time UI sign-in link (key in a header, never in the URL), valid for seconds.

## Versions

LiteLLM v1.110.3 (`ghcr.io/berriai/litellm@sha256:1050d2c9ca58399de9cd51df6eb503812af697717f54d2f08c1c30589c75e1f7`), Postgres 17.
