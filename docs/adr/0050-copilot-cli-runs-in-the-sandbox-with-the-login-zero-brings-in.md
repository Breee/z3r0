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
5. Amended 2026-11-26 (#173, PoC 23): in a workspace that uses the model proxy (ADR 0049) the sandbox holds no Copilot login. There the CLI is started against the proxy, the way its own help documents for a provider of your own: `COPILOT_PROVIDER_BASE_URL` is the proxy, `COPILOT_PROVIDER_API_KEY` the sandbox's proxy key, `COPILOT_OFFLINE=true`, so it asks GitHub for nothing. The binary is named in the sandbox's `models` rule and reaches the proxy only; rule `copilot-cli` is not added, and one left from before the workspace had models is removed at `zero up`. A sandbox created before that still holds the placeholder login until `zero rm && zero up` (zero says so). What the CLI gets as its key is the sandbox's placeholder for the proxy key; the key itself stays outside (#162, PoC 20). The CLI needs a model on every run: `zero copilot --model PROVIDER/MODEL`; without it zero refuses before the sandbox is started or created.
6. `zero claude` is not decided here: Claude Code has no documented Copilot login, and which login to use is open.

## Consequences

- The screen comes raw from the sandbox, as with `zero opencode` (ADR 0048).
- The CLI's built-in GitHub MCP server cannot connect (its host is not opened); the CLI prints a warning.
- A sandbox made from an older image has no Copilot CLI: `zero rm && zero up`.
- The image grows by about 224 MB.
- zero's instructions and skills settings are not told to the Copilot CLI (opencode only).
- Through the model proxy the features the CLI runs on GitHub's servers are off (its help: sign-in, web tools, GitHub MCP server, cloud agent). The sandbox holds only a placeholder for the proxy key (#162, PoC 20).
