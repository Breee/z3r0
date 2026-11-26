# PoC 23 — GitHub Copilot CLI in a sandbox against the model proxy (#173)

Question: can the Copilot CLI in a sandbox work through the model proxy (LiteLLM) with the sandbox's own key, with no GitHub login in the sandbox?

Result (2026-11-26, Copilot CLI 1.0.116, LiteLLM v1.110.3, OpenShell 0.1.8, opencode 2.2.6): **yes.** `run.sh`: 9 PASS, 0 FAIL.

## Sources

The CLI's own help, read in the sandbox: `copilot help environment`, `copilot help providers`, `copilot help providers-local`, `copilot help providers-env`.

- A model provider of your own is set with `COPILOT_PROVIDER_BASE_URL` (type `openai` by default, for any OpenAI-compatible address) and `COPILOT_PROVIDER_API_KEY`. The help calls the variables "legacy" and names them for scripts and single runs; "GitHub authentication isn't required".
- The other way is a `providers.json` the CLI writes itself through `/model`. Its format is not in the help, so it is not used here.
- "A model is required": `--model`, `COPILOT_MODEL` or `COPILOT_PROVIDER_MODEL_ID`.
- `COPILOT_OFFLINE=true` skips all network access but the provider: GitHub sign-in, web tools, the GitHub MCP server, telemetry, update.

## Chain

```
copilot (sandbox, /usr/local/bin/copilot)
  -> http://host.openshell.internal:18096/v1   egress rule bound to the binary
  -> LiteLLM (zero-parts, key of the sandbox)
  -> GitHub Copilot (login of the person, held by LiteLLM)
```

## Result

| Check | Result |
|---|---|
| Without an egress rule for the binary: no answer | PASS |
| With the rule: prompt answered through the proxy (`pong`) | PASS |
| Without `--model`: no answer, `BYOK providers require an explicit model.` | PASS |
| Tool use: reads a file and gives its content | PASS |
| Tool use: runs a shell command | PASS |
| `api.githubcopilot.com` not reachable from the sandbox | PASS |
| `api.github.com` not reachable from the sandbox | PASS |
| No GitHub token in the sandbox environment | PASS |
| No Copilot login variable in the sandbox | PASS |

## Findings

1. The Copilot CLI needs no Copilot login of its own in a workspace that uses the model proxy. The earlier plan to bring the login back into such a sandbox is dropped.
2. The rule must name the binary: opencode's rule for the proxy does not cover `/usr/local/bin/copilot`.
3. The model has to be named on every run. The workspace's list can be a wildcard, so zero cannot pick one; `--model` stays a plain option for now.
4. The key is the sandbox's LiteLLM key, readable in the sandbox as it is for opencode (#170 looks at holding it outside).

## Limits

- Only `-p` runs; the interactive screen was not started.
- One model (`github_copilot/claude-haiku-4.5`). The name is not one the CLI knows, so it uses its safe defaults for limits; `COPILOT_PROVIDER_MODEL_ID` was not tried.
- GitHub's documentation page on own models was not read; the help output of the installed version is the source.
- Features that run on GitHub's servers (cloud agent, code review, the GitHub MCP server) do not work this way, by the help text. Not tried.

## Run

Needs the model proxy of zero running (`zero up` in a workspace with `models`) and that workspace's sandbox.

```
pocs/23-copilot-cli-proxy/run.sh            # SANDBOX=zero-developer MODEL=github_copilot/claude-haiku-4.5
```

The script adds a rule `copilot-probe` to the sandbox and removes it at the end.
