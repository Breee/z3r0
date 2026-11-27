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
2. Without limits in the list opencode assumes 200,000 / 32,000 tokens. The proxy reports the real figures; the run prints the largest (`largest=github_copilot/gpt-astra-latest:922000/128000`). Since #171 zero writes them for each model that has both.
3. The proxy marks each model with a mode (run: `modes=chat:43,completion:1,embedding:3,responses:21`). zero keeps `chat`, `responses` and models without a mode; the rest cannot chat. Four models come without limits (printed by the run).
4. Seen while writing the PoC, not checked by `run.sh`: `python3` in the sandbox got no connection to the proxy, `curl` (named in the `models` rule) does.

## Limits

- Tested with `opencode --standalone` and project config files in a scratch folder. The opencode docs say config files normally reload automatically (https://opencode.ai/docs/models/, Caveats); for the running background service this was not tested, zero restarts it at `zero up`.
- The sandbox key is a curl argument inside the sandbox during the run.
- Ollama and LM Studio were pointed at the address ending in `/v1`, as their docs ask; their discovery paths were probed at the root only.
- opencode 2.2.6 only. A plugin that registers models was not tried.
- Whether the limits change opencode's behaviour was not measured.

## Run

Needs a sandbox of a workspace that uses the model proxy, and the proxy running.

```
pocs/21-opencode-model-discovery/run.sh
```
