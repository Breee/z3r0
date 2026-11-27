---
title: Model providers
weight: 4
description: Where the model login lives for opencode, Copilot CLI and Claude Code, without and with the model proxy.
---

A tool in the sandbox needs a model. There are two setups. In both, no provider credential is readable inside the sandbox: the sandbox holds a placeholder, and the real value is put in outside, on the way to the one host it belongs to.

| | Without the model proxy (default) | With the model proxy |
|---|---|---|
| Set in | nothing | `parts.models`, `providers`, `models` in `~/.config/zero/config.yaml` |
| You log in | on the host: `opencode auth login`, GitHub Copilot | once in the proxy's UI: connect GitHub Copilot |
| The sandbox holds | a placeholder for the Copilot token | a placeholder for its own proxy key |
| Model hosts the sandbox reaches | `api.githubcopilot.com`, `opencode.ai`, `models.opencode.ai` | none; only the proxy on your machine, and of it only the model list and the model calls (chat, responses, messages, token count, embeddings) |
| `zero opencode`, `zero web` | yes | yes |
| `zero run` | yes | only with `model: litellm/github_copilot/...` in the repository's `opencode.json`; opencode's own default model is not reachable here ([#176](https://github.com/Breee/z3r0/issues/176)) |
| Copilot inside `zero vscode` | yes | no: it has no login and no way out |
| `zero copilot` | yes | yes, with `--model` |
| `zero claude` | no | yes, with `--model` |
| Needs | the host's opencode with a Copilot login | Docker for the proxy |

`zero doctor` says what is missing: `model login` for the first setup, `model proxy runtime` and `model proxy login` for the second. When every workspace uses the model proxy, `model login` is `na`: no login of the host's opencode is needed then.

## Without the model proxy

Log in once on the host:

```
opencode auth login
```

Choose GitHub Copilot. `zero up` takes that login when it creates the sandbox; a sandbox created before the login has none (`zero rm`, then `zero up`).

**opencode** starts with its own default, a free model with a daily limit. `/models` lists the models, the Copilot ones among them; or set `model` in an `opencode.json` of the repository.

**Copilot CLI** uses the same login:

```
$ zero -c developer copilot -p "answer with the single word: pong"
copilot: /usr/local/bin/copilot may reach api.githubcopilot.com
pong
```

The output is shortened: the CLI also prints its usage figures. The first line appears once, when zero opens that host for the program. The CLI's built-in GitHub tools do not connect: `api.github.com` stays closed for it.

**Claude Code** is not offered in this setup. It would need an Anthropic login, and zero brings none in.

## With the model proxy

zero starts a LiteLLM proxy on your machine and the sandbox reaches models only through it. How to switch it on, and what `zero up` prints, is in [Policy: Models through the model proxy](../policy/#models-through-the-model-proxy).

All three tools then use the models the workspace lists. Copilot CLI and Claude Code need the model named each time:

```
$ zero -c developer copilot --model github_copilot/claude-haiku-4.5 -p "what is 30 plus 12? answer with the number only"
42
$ zero -c developer claude --model github_copilot/claude-haiku-4.5 -p "what is 17 plus 25? answer with the number only"
42
```

In opencode the models appear as `litellm/github_copilot/...` in `/models`.

## Limits

- The model proxy sees every prompt and every answer and holds your Copilot connection. Two LiteLLM releases were compromised in 2026 (March and October). zero pins the proxy's image by digest ([ADR 0049](https://github.com/Breee/z3r0/blob/main/docs/adr/0049-zero-cp-is-the-one-front-door-one-config-file.md)).
- The proxy runs under Docker, a daemon with root rights, until the podman on the host can run health checks ([ADR 0051](https://github.com/Breee/z3r0/blob/main/docs/adr/0051-opt-in-parts-run-under-docker-until-podman-has-health-checks.md)).
- Anthropic supports Claude Code behind a gateway with Claude models only. Shown here: a Claude model served by Copilot. Another model was not tried ([ADR 0052](https://github.com/Breee/z3r0/blob/main/docs/adr/0052-claude-code-runs-in-the-sandbox-through-the-model-proxy.md)).
- Only GitHub Copilot is a provider today ([#177](https://github.com/Breee/z3r0/issues/177)).
- No default model per workspace: `--model` each time for `zero copilot` and `zero claude` with the proxy ([#176](https://github.com/Breee/z3r0/issues/176)).
- Run for this page: the three quoted commands, `zero up`, `zero doctor`, and opencode with a `litellm/` model inside the sandbox. Not run: the screens of `zero copilot` and `zero claude` in a terminal, `zero run` with a model set in the repository, VS Code with the proxy.
- Instructions and skills of a workspace are told to opencode only.
- The personal opencode configuration of the host (plugins, MCP servers, other providers) is not carried into the sandbox ([#153](https://github.com/Breee/z3r0/issues/153)).
