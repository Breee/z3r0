# PoC 25 — LiteLLM's web UI below a sub-path behind one loopback port (#163)

Question: can zero's UI listener carry LiteLLM's own UI below a sub-path, so there is one address, and is that port out of a sandbox's reach?

Result (2026-11-27, LiteLLM v1.110.3 with Postgres, Docker, OpenShell 0.1.8): **yes for the sub-path, no for signing in without typing.** `run.sh`: 10 PASS, 1 NOTE.

## Sources

- Sub-path: `SERVER_ROOT_PATH`, https://docs.litellm.ai/docs/proxy/custom_root_ui
- Sign-in from a front proxy: https://docs.litellm.ai/docs/proxy/custom_sso ("The custom UI sign-in hook for an upstream authentication proxy requires Enterprise.") -> caveat C59

## Chain

browser -> `127.0.0.1:18111` (`front/main.go`, stands in for zero-cp's UI listener; checks the Host name, passes on only `/litellm/`) -> LiteLLM on `127.0.0.1:4000` started with `SERVER_ROOT_PATH=/litellm`.

## Result

| Check | Result |
|---|---|
| LiteLLM answers below `/litellm` | PASS |
| LiteLLM at the old place, without the sub-path | NOTE: still answers (http 200) |
| Health and UI page through the front port | PASS |
| The page names 37 script and style files, all below `/litellm`, all load through the front port | PASS |
| A path outside `/litellm` is not passed on (404) | PASS |
| Another Host name is refused (403) | PASS |
| Model list without a key through the front port (401) | PASS |
| The front port from inside sandbox `zero-developer` | PASS: not reachable (000) |
| Browser, by hand: `http://127.0.0.1:18111/litellm/ui/` | LiteLLM's login page, at `/litellm/ui/login/?redirect_to=...` on the front port |

## Findings

1. The stock image serves its UI below the sub-path with one setting; no rebuilt image.
2. LiteLLM keeps answering without the sub-path too. The front port must pass on only the sub-path, and LiteLLM's own port stays on loopback.
3. A sandbox reaches host ports only by a rule; no rule names this port.
4. Signing the user in from the front port is not possible with open-source LiteLLM by a documented way (C59). LiteLLM's own login stays.

## Limits

- The login form was seen, a login was not typed in the browser through the front port.
- LiteLLM's API below the sub-path was only touched for health and the refused model list; zero's proxy (port 18096) runs without the sub-path today, moving it changes the sandbox's base address.
- Websocket and streaming calls through the front port not tried.
- Docker only (ADR 0051).

## Run

```
pocs/25-zero-ui/run.sh          # KEEP=1 leaves it running: http://127.0.0.1:18111/litellm/ui/
```
