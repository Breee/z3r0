# PoC 21 — how opencode gets its model list from the model proxy (#171)

Question: can opencode in a sandbox find the models of the LiteLLM proxy by itself, or must zero write the list?

Result (2026-11-26, opencode 2.2.6, LiteLLM v1.110.3, OpenShell 0.1.8): **zero must write the list.** No discovery way of opencode lists the proxy's models. New from this run: the proxy reports token limits per model, and zero can write them into the list. `run.sh`: 17 PASS, 0 FAIL.

## Sources

- Discovery exists only for the built-in runtimes vLLM, LM Studio and Ollama: https://opencode.ai/docs/models/#vllm, https://opencode.ai/docs/providers/#runtimes
- vLLM discovery "includes only model cards owned by `vllm`" (same page).
- Another OpenAI-compatible endpoint: "use the custom provider recipe and list its models explicitly": https://opencode.ai/docs/models/#compatible, https://opencode.ai/docs/providers/#custom
- A model opencode does not know gets a 200,000-token context limit and a 32,000-token output limit: https://opencode.ai/docs/models/#aliases
- Claude models behind LiteLLM need `compatibility.cacheFormat: anthropic`: https://opencode.ai/docs/models/#compatibility
- Where config files are read from: https://opencode.ai/docs/config/#locations

## Result

| Check | Result |
|---|---|
| Proxy `/v1/models` | 68 models, all `owned_by: openai`, 64 with `max_input_tokens` and `max_output_tokens` |
| Proxy `/api/v1/models` (LM Studio), `/api/tags` (Ollama) | 404 |
| opencode `vllm` provider pointed at the proxy | lists no model of the proxy |
| opencode `ollama` provider pointed at the proxy | lists no model of the proxy |
| opencode `lmstudio` provider pointed at the proxy | lists no model of the proxy |
| Custom provider with a written list (2 models, with limits) | both listed, a model not in the file is not listed |
| Prompt through that provider | answered (42) |
| Third model added to the file | listed at the next start of opencode |

## Findings

1. vLLM discovery reads `/v1/models` but keeps only models owned by `vllm`; LiteLLM says `openai` for every model. LM Studio and Ollama discovery ask paths the proxy does not have.
2. Without limits in the list opencode assumes 200,000 / 32,000 tokens. The proxy reports the real figures, for example 922,000 / 128,000 for `github_copilot/gpt-astra-latest`. zero's list carries no limits today.
3. Four models come without limits: `gpt-41-copilot` and three embedding models. Embedding models are not chat models.
4. Only programs named in the sandbox's `models` rule reach the proxy: `curl` does, `python3` does not.

## Limits

- Tested with `opencode --standalone` and project config files in a scratch folder. Whether the running background service re-reads a changed config was not tested; zero restarts it at `zero up`.
- opencode 2.2.6 only. A plugin that registers models was not tried.
- Whether the limits change opencode's behaviour was not measured.

## Run

Needs a sandbox of a workspace that uses the model proxy, and the proxy running.

```
pocs/21-opencode-model-discovery/run.sh
```
