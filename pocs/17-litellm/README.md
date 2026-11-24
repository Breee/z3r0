# PoC 17 — LiteLLM as a model proxy next to the sandbox: what it costs to run (#156)

Question: is LiteLLM light enough to run on a developer machine as the one place that holds model
provider logins, outside the sandbox?

Result (2026-11-25, LiteLLM v1.110.3, Docker 29.5.2): **it runs as one container without a database,
but it is not small**: 1.36 GB image, 17 to 26 s to start, about 630 MiB memory doing nothing.
Copilot as backend: not shown yet (needs the device login of a person, see below).

## Chain

```
client (curl; later opencode, Claude Code in the sandbox)
  -> LiteLLM 127.0.0.1:4000 (key from run.sh, models from config.yaml)
     -> claude-stand-in: backend.py of this PoC (answers "pong" at once)
     -> claude-copilot:  GitHub Copilot (token file in a mounted folder)
```

Everything is in `compose.yaml` and `config.yaml`; `run.sh` makes the key, starts compose and measures.

## Result

Three runs of `COMPOSE="docker compose" ./run.sh`:

| Check | Result |
|---|---|
| Image, pinned by digest | 1 362 549 475 bytes (Docker); 1 388 033 968 (podman, same digest) |
| Start until `/health/liveliness` answers | 17.4 s, 17.7 s, 25.8 s |
| Memory, nothing to do | 622 to 641 MiB, CPU 0.2 % |
| Memory after 100 requests | 673 to 696 MiB |
| Time per request, 50 in a row | backend alone 6 to 11 ms; through the proxy 22 ms (run 1), 62 and 69 ms (runs 2, 3) |
| Request in OpenAI form (`/v1/chat/completions`) | answered by the stand-in |
| Request in Anthropic form (`/v1/messages`) | answered by the stand-in |
| Request without the key | 401 |
| Key in the proxy's log | no |
| Database | none; models and key come from `config.yaml` and the environment |
| Copilot as backend | not shown: before the login the proxy answers 400 and names the login command |

## Findings

- No database is needed for this use. Without one LiteLLM has no budgets and no keys per user.
- A request in Anthropic form for an `openai/` model is sent on as a Responses API call
  (`.../responses`), not as a chat completion. The stand-in answers both forms for that reason.
- The time added per request differed between runs (11 ms, then 56 to 61 ms). Run 1 had a
  stand-in that answered chat completions only; why runs 2 and 3 are slower is not looked into.
- The Copilot login is LiteLLM's own device login. The proxy cannot run it; it must be done once
  by a person, and the token file lands in the mounted folder.

## Limits

- Measured with Docker running as root. `podman compose down` fails on this machine (caveat C57,
  Ubuntu 26.04), so no complete run with podman exists; start and requests work with podman too.
- The stand-in is not a model: the numbers say what the proxy costs, not what a model call costs.
- 50 requests one after the other with curl; no parallel load.
- The container holds the master key (environment) and, after the login, the Copilot token file
  (`~/.local/state/zero-poc17/copilot`). It can reach every host; no egress limit was set.
- LiteLLM had two compromised releases in 2026 (1.82.7/1.82.8 in March, v1.107.0 in October,
  Docker tags included). The image here is pinned by digest for that reason.
- The podman policy on this machine got an entry for `ghcr.io/berriai` to pull the image.
- Anthropic does not support Claude Code with models that are not Claude behind a gateway.
- Not run: a sandbox, opencode or Claude Code as client.

## Run

```
./run.sh                              # podman compose
COMPOSE="docker compose" ./run.sh     # Docker
```

Copilot login (once, in a terminal):

```
KEEP=1 COMPOSE="docker compose" ./run.sh
export LITELLM_MASTER_KEY="$(cat ~/.local/state/zero-poc17/key)" COPILOT_DIR=~/.local/state/zero-poc17/copilot
docker compose exec litellm python -c "from litellm.llms.github_copilot.authenticator import Authenticator; Authenticator().get_access_token()"
curl -s -H "authorization: Bearer $LITELLM_MASTER_KEY" -H 'content-type: application/json' \
  -d '{"model":"claude-copilot","max_tokens":16,"messages":[{"role":"user","content":"say pong"}]}' \
  http://127.0.0.1:4000/v1/messages
docker compose down
```
