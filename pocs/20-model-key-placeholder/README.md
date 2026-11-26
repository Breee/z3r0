# PoC 20 — the sandbox's model proxy key held outside the sandbox (#170)

Question: can the LiteLLM key of a sandbox stay outside the sandbox, with only a placeholder inside, the way the forge token does?

Result (2026-11-26, OpenShell 0.1.8, LiteLLM v1.110.3, opencode 2.2.6, Copilot CLI 1.0.116): **yes**, for clients that send the key as `Authorization: Bearer`. `run.sh`: 10 PASS, 0 FAIL, 1 NOTE.

## Sources

OpenShell v0.1.8 docs:

- `docs/how-it-works/providers/profiles.mdx`: `credential_binding` (sandbox policy only; the provider must be attached and its profile must have no endpoints; a placeholder sent outside its binding gets 403), "Attach and detach providers" (works on a running sandbox; states ready, withheld, revoked, failed).
- `docs/how-it-works/providers/overview.mdx`: the sandbox gets an opaque placeholder in its environment, the supervisor swaps it on the bound endpoint.
- `docs/how-it-works/policies/schema.mdx`: `credential_binding.provider`.
- `openshell sandbox provider attach|detach|status --help`.

zero already uses the same mechanism for the forge token and the push gate (`internal/zero/egress.go`).

## Chain

```
client in the sandbox --Bearer <placeholder>--> supervisor (swaps in the key) --> LiteLLM on the host
```

`profile.yaml` declares a credential without endpoints. `run.sh` mints a LiteLLM key, stores it in a provider, attaches the provider to the running sandbox and binds it to the model proxy endpoint of the sandbox's `models` rule.

## Result

| Check | Result |
|---|---|
| Sandbox environment holds a placeholder, not a LiteLLM key | PASS |
| curl: model list with the placeholder as Bearer | PASS |
| curl: chat completion answered | PASS |
| Placeholder in the `x-api-key` header | NOTE: refused by OpenShell, `credential_placement_mismatch` |
| No key | PASS: 401 from the proxy |
| Copilot CLI with the placeholder as `COPILOT_PROVIDER_API_KEY` | PASS: answers |
| opencode (`--standalone`) with the placeholder as `LITELLM_API_KEY` | PASS: answers |
| Placeholder sent to the forge | PASS: 403 by OpenShell |
| Forge placeholder still works on the forge | PASS |
| After policy restore and detach: no model list | PASS |

## Findings

1. Order matters: attach the provider, then set the rule with the binding, then wait for the provider status. `attach --wait` alone fails with `withheld`, since no rule binds the credential yet, and a rule can only name an attached provider.
2. The credential is swapped only where the profile says (here: `authorization`, bearer). A client that sends `x-api-key` is refused. Claude Code must use `ANTHROPIC_AUTH_TOKEN` (Bearer), not `ANTHROPIC_API_KEY`, or the profile needs a second placement.
3. The placeholder is worthless elsewhere: any other host refuses it.
4. No sandbox recreate is needed to move an existing sandbox to this.

## Limits

- opencode ran with `--standalone`; its background service with the placeholder in the service environment was not run.
- Claude Code not run.
- Key rotation through `openshell provider update` not run.
- One sandbox; `-p` style runs only.

## Run

Needs the model proxy of zero on 127.0.0.1:18096 and a sandbox of a workspace with `models`.

```
pocs/20-model-key-placeholder/run.sh        # SANDBOX=zero-developer by default
```
