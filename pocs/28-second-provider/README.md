# PoC 28 — a second model provider behind the model proxy (#177)

Question: is a second provider one entry in the proxy's config, with its key in no file?

Result (2026-11-27, LiteLLM v1.110.3 with Postgres, Docker): **yes.** `run.sh`: 9 PASS, 0 FAIL, 2 NOTE. The provider here is a stand-in that speaks the OpenAI form and wants an API key; no real provider was called.

## Sources

- https://docs.litellm.ai/docs/providers/openai_compatible (`model: openai/<name>`, `api_base`, `api_key`)
- https://docs.litellm.ai/docs/wildcard_routing, https://docs.litellm.ai/docs/proxy/forward_client_headers (wildcard entry with `api_base`)
- https://docs.litellm.ai/docs/proxy/model_discovery (`check_provider_endpoint`)
- https://docs.litellm.ai/docs/proxy/configs (`os.environ/NAME`)

## Chain

```
client with a proxy key -> LiteLLM (entry acme/* -> openai/* at the stand-in, api_key: os.environ/ACME_API_KEY) -> stand-in provider
```

The key's value is made by `run.sh`, lives in its shell only and reaches the proxy container through the environment of the compose command.

## Result

| Check | Result |
|---|---|
| Prompt answered by the second provider, two of its models | PASS |
| Model list of a proxy key with `acme/*` | `acme/small`, `acme/large` (read from the provider) |
| Proxy key without the provider in its list | refused |
| Provider called without its key | 401 |
| Key's value in a file of the PoC, the proxy log, an answer | no |
| Proxy started without the key | starts; a call gives an authentication error, no answer |
| Key back, proxy made anew | answers again |

## Findings

1. One entry is enough: `model_name: "acme/*"`, `model: "openai/*"`, `api_base`, `api_key: os.environ/NAME`. The same form with `anthropic/*` or `openrouter/*` and no `api_base` is in LiteLLM's docs for real providers; not run here.
2. The proxy reads the provider's model list for the wildcard, also for an endpoint that only speaks the OpenAI form. A sandbox key limited to `acme/*` sees exactly these models.
3. A missing key does not stop the proxy. The provider's models then give an authentication error at the first call; other providers are not touched.
4. The key is read at the container's start. A new key means the proxy is made anew (about 20 s, PoC 27).

## Limits

- Stand-in provider, no real one; no key of a real provider was used.
- The key is in the container's environment: whoever may run `docker inspect` on the host reads it.
- Not run from a sandbox; PoC 21, 23 and 24 showed the clients against the same proxy routes.
- Docker only (ADR 0051).

## Run

```
pocs/28-second-provider/run.sh
```
