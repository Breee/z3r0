---
status: accepted
date: 2026-11-16
deciders: bree
requirements: [Z-INV-4, Z-INV-6]
---
# 0036 — Pause ends a session; the token expiry date is only a backstop

## Context

GitLab token expiry is a date: the token dies at 00:00 UTC of that day. A token minted with "tomorrow" lives 0–24 h; after 12:00 UTC it is already inside the 12 h renewal window, so `zero serve` re-minted it every 10 minutes (each time breaking running processes, C44). OpenShell can stop and start a sandbox while keeping its files (`ai-docs/research-pause-resume.md`, #104).

## Decision

- `zero pause` closes the tunnel, stops the sandbox and revokes its forge token. Files and opencode data stay.
- `zero up` on a paused sandbox mints a new token, rotates the gateway credential, then starts the sandbox, so every process starts with the new token.
- The GitLab expiry date is today + 2 days (token lives 24–48 h). It is the backstop for a laptop that is switched off; the normal end of a token is `zero pause` or `zero rm`.
- `zero status` shows `paused`; `zero serve` upkeep skips paused sandboxes.

## Consequences

- No `zero pause` with `--vm`: the VM driver loses `/tmp`, where the checkout lives (C50). zero refuses with a message.
- Placeholder versions change when a sandbox starts. zero re-imports the Copilot credential into opencode on resume (C51).
- A sandbox that keeps running is still renewed by `zero serve`, with C44.
- Laptop off without pause: the token lives until its expiry date (up to 48 h, was up to 24 h).
- Not tested: reopening an opencode session in the web UI after resume.
