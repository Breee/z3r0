# PoC 26 — a default model for opencode, Copilot CLI and Claude Code behind the model proxy (#176)

Question: can zero name one default model per workspace, so `zero run`, `zero copilot` and `zero claude` answer without a model argument, while a repository's own setting still wins?

Result (2026-11-27, opencode 2.2.6, Copilot CLI 1.0.116, Claude Code 2.1.332, LiteLLM v1.110.3, OpenShell 0.1.8): **yes.** `run.sh`: 10 PASS.

## Sources

- opencode: order of config files (user, repository, closer folder, system), root field `model`: https://opencode.ai/docs/config
- opencode: `--model` for one run, `model` as the default: https://opencode.ai/docs/models
- Copilot CLI: `copilot help providers-env` ("A model is required for BYOK. Set COPILOT_MODEL, COPILOT_PROVIDER_MODEL_ID, or --model")
- Claude Code: `ANTHROPIC_MODEL`, `--model`, `model` setting: https://code.claude.com/docs/en/model-config

## Chain

sandbox `zero-developer` (workspace with the model proxy) -> `host.openshell.internal:18096` (LiteLLM, started by zero) -> GitHub Copilot.

The script works only in `/tmp/poc26` in the sandbox: its own home folder with a copy of the opencode config zero wrote, plus a `model` line; a `repo` folder with its own `opencode.json`. zero's config and the running opencode service are not touched.

## Result

| Check | Result |
|---|---|
| opencode, `model` in the user config: prompt answered without a model argument | PASS |
| That model is the one used | PASS |
| opencode, `model` in the repository's `opencode.json`: answered, the repository's model is used, the user config's model is not | PASS |
| opencode, `--model`: wins over both | PASS |
| Copilot CLI, `COPILOT_MODEL` in the environment, no `--model`: answered | PASS |
| Copilot CLI, no model at all: refused ("require an explicit model") | PASS |
| Claude Code, `ANTHROPIC_MODEL` in the environment, no `--model`: answered | PASS |

## Findings

1. A `model` line in the user config that zero writes is opencode's default. A repository's `opencode.json` wins over it, `--model` wins over both. This matches the docs.
2. Copilot CLI and Claude Code take the default from an environment variable. zero can also pass `--model` itself, which is the form it already runs (PoC 23, PoC 24).
3. Not every model the proxy lists answers: `github_copilot/gpt-5.4` was refused by Copilot ("not available for integrator vscode-chat"). zero can check the name of a default model against the workspace's list, not whether the provider serves it.

## Limits

- opencode ran with `--standalone` and a copied config; the background service with the `model` line is shown in zero's live check, not here.
- Two Claude models served by Copilot; no other provider.
- Only `-p` runs; no interactive screen.

## Run

```
pocs/26-default-model/run.sh
```

Needs a running sandbox of a workspace with `models` (default `zero-developer`) and the model proxy with a connected Copilot account.
