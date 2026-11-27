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
| Model hosts the sandbox reaches | `api.githubcopilot.com`, `opencode.ai`, `models.opencode.ai` | none; only the proxy on your machine, and of it only the model list and the model calls |
| `zero opencode`, `zero run`, `zero web` | yes | yes |
| `zero copilot` | yes | yes, with `--model` |
| `zero claude` | no | yes, with `--model` |
| Needs | the host's opencode with a Copilot login | Docker for the proxy |

`zero doctor` says what is missing: `model login` for the first setup, `model proxy runtime` and `model proxy login` for the second.

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

- Only GitHub Copilot is a provider today. Other providers of LiteLLM (API keys, clouds, local models) are not built into zero's configuration.
- Anthropic does not support Claude Code with models that are not Claude models behind a gateway. Shown here: a Claude model served by Copilot. Another model was not tried.
- Two LiteLLM releases were compromised in 2026 (March and October). zero pins the proxy's image by digest; the proxy holds your Copilot connection and runs under Docker.
- The proxy needs Docker until the podman on the host can run health checks.
- The screens of `zero copilot` and `zero claude` in a terminal were not started for this page; the lines above come from runs with `-p`.
- Instructions and skills of a workspace are told to opencode only.
- The personal opencode configuration of the host (plugins, MCP servers, other providers) is not carried into the sandbox.
