---
status: accepted
date: 2026-11-27
deciders: bree
requirements: [Z-INV-2]
---

# 0054 — A workspace can name its default model

## Context

ADR 0048 decided that zero sets no model. With the model proxy (ADR 0049) opencode's own default model is not reachable from the sandbox, so `zero run` answered only when the repository named a model, and `zero copilot` and `zero claude` needed `--model` on every run. PoC 26 showed how each tool takes a default (#176).

## Decision

1. A workspace may carry `model: PROVIDER/MODEL` in zero's config. It must be one of the workspace's `models` (by name, or covered by a `PROVIDER/*` entry), so it exists only with the model proxy. A name outside the list fails at load.
2. zero writes it as `model` into the opencode user config it renders in the sandbox. A repository's `opencode.json` and `--model` still win, by opencode's own order.
3. `zero copilot` and `zero claude` get `--model` from it when the command line names none.
4. Without the line nothing changes: ADR 0048 stands, zero sets no model.

## Consequences

- `zero run` answers through the model proxy without a model in the repository.
- zero checks the name against the workspace's list, not whether the provider serves that model: in PoC 26 one listed Copilot model was refused by Copilot itself. The tool then prints the provider's refusal.
- A workspace without the model proxy still has no way to set a default from zero's config.
