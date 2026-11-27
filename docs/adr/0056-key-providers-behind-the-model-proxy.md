---
status: accepted
date: 2026-11-27
deciders: bree
requirements: [Z-INV-2, Z-INV-4]
---

# 0056 — Providers with an API key behind the model proxy; the key comes from a command

## Context

The model proxy (ADR 0049) knew one provider, GitHub Copilot. A second provider should be one entry in zero's config, and its key must not be in that file, in a repository or in a sandbox. PoC 28 showed a provider with a key behind the proxy, with the key handed over through the environment only (#177).

## Decision

1. `providers` accepts `openai`, `anthropic` and `openrouter` next to `github_copilot`.
2. Each of the three needs an entry in `provider_keys`: a command that prints the key, for example `openrouter: pass show OPENROUTER_API_KEY`. A missing entry, or an entry for a provider that is not listed or takes no key, fails when the config is loaded.
3. zero runs the command without a shell at every `zero up` of a workspace with models. The first printed line is the key. The command is split at spaces; one with quotes, pipes or variables fails at load. It has 30 seconds. An error names the provider, not the command or what it printed.
4. The key reaches the proxy only through the environment of the compose command. zero writes it to no file. It keeps the key's SHA-256 in `parts/keys.sha` to see a change; the file is written once the proxy answers with the new key.
5. The proxy config gets one entry per provider (`P/*`, key from the container's environment). A workspace names the models as before (`openrouter/*` or a full name).
6. A changed key makes the proxy anew.
7. `zero doctor` runs the commands and reports `model provider keys`. It shows no value.

## Consequences

- The key is readable with `docker inspect` by everyone who may use Docker on the host (ADR 0051).
- A command that asks for a passphrase asks at `zero up`.
- Making the proxy anew takes about 20 s; every sandbox that uses the proxy waits (PoC 27, PoC 28).
- A provider without its key does not stop the proxy; calls to that provider fail, others work (PoC 28).
- The entry forms for the three providers are taken from LiteLLM's docs. Run was a stand-in provider in OpenAI form (PoC 28); and `openrouter` with a real key (one free model answered through opencode in a sandbox); `openai` and `anthropic` were not run.
- Other OpenAI-compatible addresses and local runtimes are not built.
- zero checks a model name against the workspace's list, not whether the provider serves it (ADR 0054).
