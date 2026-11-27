---
status: accepted
date: 2026-11-25
deciders: bree
requirements: [Z-INV-1, Z-INV-4]
---
# 0048 — `zero opencode` and `zero run` start opencode inside the sandbox over ssh

## Context

`zero opencode` (before: `zero attach`) and `zero run` started the host's opencode with `--server URL` and the sandbox folder. opencode 2.x resolves that folder on the host, so both fail (`ENOENT ... chdir ... -> '/tmp/zero/<folder>'`, C38, upstream anomalyco/opencode#47665, open on 2026-11-25). Since ADR 0037 they also had no credential for the server. #152.

## Decision

1. `zero opencode` (same as `zero opencode --tui`) runs `opencode` inside the sandbox, in the workspace's first repository folder, over ssh with a terminal. It needs a terminal; without one it refuses and names `zero run` and `zero opencode --web`.
2. `zero opencode --web` does what `zero web` does (pairing link, ADR 0037). `zero web` stays.
3. `zero run PROMPT` runs `opencode run PROMPT` inside the sandbox the same way, without a terminal.
4. The host's opencode is no longer started by zero for these commands. Nothing is worked around on the host (no mirrored paths).

## Consequences

- The screen of `zero opencode` and the answers of `zero run` come from the sandbox unfiltered, like any ssh session or the VS Code terminal: a program in the sandbox can write escape sequences to the terminal while the command runs. zero's own prints stay filtered (#140).
- The host opencode's settings (theme, keybindings, plugins, agents) do not apply; the sandbox's opencode has its own.
- No credential enters or leaves the sandbox for this. The model login is the one the sandbox has (the GitHub Copilot login copied at creation). zero sets no model (decided 2026-11-25; since ADR 0054 a workspace with the model proxy can name one): opencode's default is used, a free model with a daily limit. The user picks another with `/models` in the UI or with `model` in an `opencode.json` of the repository; the GitHub Copilot models are in the list when the host had that login. `zero run` has no option for the model and uses the default or the repository's setting.
- The opencode version of the TUI is the image's, not the host's.
- If upstream fixes #47665, the host TUI could be offered again; it would need a credential for the server (ADR 0037).
