# PoC 17 — LiteLLM as a model proxy next to the sandbox: what it costs to run (#156)

Question: is LiteLLM light enough to run on a developer machine as the one place that holds model
provider logins, outside the sandbox?

Result (2026-11-25, LiteLLM v1.110.3, Docker 29.5.2): **it runs as one container without a database,
but it is not small**: 1.36 GB image, 17 to 26 s to start, about 630 MiB memory doing nothing.
Copilot as backend: shown with the database variant (web UI, each user connects their own GitHub account).

## Chain

```
client (curl; later opencode, Claude Code in the sandbox)
  -> LiteLLM 127.0.0.1:4000 (key from run.sh, models from config.yaml)
     -> claude-stand-in: backend.py of this PoC (answers "pong" at once)
     -> github_copilot/*: GitHub Copilot (per-user GitHub login, kept in the database)
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
| Copilot as backend (with database, one run) | `github_copilot/claude-haiku-4.5` answered `pong` in OpenAI form and in Anthropic form; `/v1/models` lists 68 `github_copilot/` models |

## Findings

- No database is needed for this use. Without one LiteLLM has no budgets and no keys per user.
- A request in Anthropic form for an `openai/` model is sent on as a Responses API call
  (`.../responses`), not as a chat completion. The stand-in answers both forms for that reason.
- The time added per request differed between runs (11 ms, then 56 to 61 ms). Run 1 had a
  stand-in that answered chat completions only; why runs 2 and 3 are slower is not looked into.
- The Copilot credential is declared in `config.yaml` (`credential_list`, `github_copilot_auth_type: per_user_oauth`).
  LiteLLM's provider page documents `credential_list` only for GitHub App auth; this form works on v1.110.3.
  Each user connects in the web UI; that needs the database.

## Limits

- Measured with Docker running as root. `podman compose down` fails on this machine (caveat C57,
  Ubuntu 26.04), so no complete run with podman exists; start and requests work with podman too.
- The stand-in is not a model: the numbers say what the proxy costs, not what a model call costs.
- 50 requests one after the other with curl; no parallel load.
- The container holds the master key (environment) and, after the login, the Copilot token file
  (per-user variant: the GitHub token of each user, in the database). It can reach every host; no egress limit was set.
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

With web UI and Copilot (adds Postgres; LiteLLM's schema allows no other database):

```
KEEP=1 UI=1 COMPOSE="docker compose" ./run.sh
```

One run: start 35.6 s, LiteLLM idle 742 MiB, Postgres image 297 528 495 bytes, Postgres idle 41 MiB,
787 MiB after 100 requests.

1. Open http://127.0.0.1:4000/ui/, log in as `admin` with the key in `~/.local/state/zero-poc17/key`.
2. Models + Endpoints > LLM Credentials > Your connections > Connect on `github-copilot`, enter the code at GitHub.
3. Ask a model, for example `github_copilot/claude-haiku-4.5`, at `/v1/chat/completions` or `/v1/messages`.

Stop:

```
S=~/.local/state/zero-poc17
export LITELLM_MASTER_KEY="$(cat $S/key)" COPILOT_DIR=$S/copilot PG_PASSWORD="$(cat $S/pg)"
docker compose -f compose.yaml -f compose.ui.yaml down
```
