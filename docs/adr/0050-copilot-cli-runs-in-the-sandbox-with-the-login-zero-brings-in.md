---
status: accepted
date: 2026-11-26
deciders: bree
requirements: [Z-INV-1, Z-INV-2, Z-INV-4]
---

# 0050 — `zero copilot` runs the GitHub Copilot CLI in the sandbox with the Copilot login zero already brings in

## Context

Users want other agent tools than opencode in the same sandbox. The GitHub Copilot CLI is one binary and accepts a GitHub token in `COPILOT_GITHUB_TOKEN`. Tried on 2026-11-26 with Copilot CLI 1.0.116: the token zero takes from the host's opencode login is accepted, also as the placeholder inside a sandbox, and one host is enough for an answer (#149).

## Decision

1. `zero copilot [ARG...]` starts the Copilot CLI inside the sandbox over ssh, in the first repository folder. Arguments go to the tool. Without a terminal it needs arguments (`-p PROMPT`).
2. Login: the Copilot login zero already holds for opencode. The sandbox passes its placeholder as `COPILOT_GITHUB_TOKEN`; the real token stays outside.
3. Network: one rule `copilot-cli`, bound to `/usr/local/bin/copilot`, for `api.githubcopilot.com` only. zero adds it at the first `zero copilot`; `zero status` shows it.
4. The tool is in the image, pinned by digest. Its own update is switched off.
5. `zero claude` is not decided here: Claude Code has no documented Copilot login, and which login to use is open.

## Consequences

- The screen comes raw from the sandbox, as with `zero opencode` (ADR 0048).
- The CLI's built-in GitHub MCP server cannot connect (its host is not opened); the CLI prints a warning.
- A sandbox made from an older image has no Copilot CLI: `zero rm && zero up`.
- The image grows by about 224 MB.
- zero's instructions and skills settings are not told to the Copilot CLI (opencode only).
