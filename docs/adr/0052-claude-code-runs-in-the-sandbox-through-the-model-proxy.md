---
status: accepted
date: 2026-11-27
deciders: bree
requirements: [Z-INV-1, Z-INV-2, Z-INV-4]
---

# 0052 — `zero claude` runs Claude Code in the sandbox, through the model proxy only

## Context

Claude Code is the third agent tool asked for next to opencode and the Copilot CLI (#149, #166). It has no documented way to use a Copilot login directly; it does document a gateway (`ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`). The developer decided: no Anthropic login, Claude Code goes through the model proxy with Copilot behind it. PoC 24 showed it works from a sandbox with the proxy key as a placeholder (sources in #166).

## Decision

1. `zero claude [ARG...]` is `up`, then Claude Code inside the sandbox over ssh, in the first repository. Arguments go to the tool; without arguments it needs a terminal.
2. It works only in a workspace that uses the model proxy (`parts.models`, `models`). Without it zero refuses and says so. No Anthropic key or subscription login is taken into a sandbox.
3. Claude Code gets the proxy address (`ANTHROPIC_BASE_URL`, without `/v1`) and the sandbox's placeholder as `ANTHROPIC_AUTH_TOKEN` (the form sent as `Authorization: Bearer`; the `x-api-key` form is refused, PoC 20). The real key stays outside (ADR 0049).
4. A model is named on every run (`--model PROVIDER/MODEL`), as for `zero copilot` in such a workspace. zero also sets it as `ANTHROPIC_MODEL` and `ANTHROPIC_DEFAULT_HAIKU_MODEL`, so calls Claude Code makes in the background use the same model.
5. The program is part of the `models` rule of the sandbox: it reaches the model routes of the proxy and nothing else. The rule is set at every `zero up` of such a workspace, not at the first `zero claude`. Its own updates (`DISABLE_UPDATES`, `DISABLE_AUTOUPDATER`), the plugin marketplace registration, telemetry and error reports are switched off; the image pins its version by digest.

## Consequences

- Anthropic does not support Claude Code with models that are not Claude models behind a gateway. Shown here: a Claude model served by Copilot. Other models: not tried.
- The screen comes raw from the sandbox, as for `zero opencode` (ADR 0048).
- Claude Code writes its session files under `~/.claude` in the sandbox; they are gone with the sandbox.
- Features that ask Anthropic's own servers (web fetch check, sign-in) do not work: `api.anthropic.com` is not reachable.
- A sandbox made from an older image has no Claude Code: `zero rm`, then `zero up`. The image grows by about 400 MB.
- Instructions and skills of the workspace are told to opencode only (#161), not to Claude Code.
