---
status: accepted
date: 2026-11-28
deciders: bree
requirements: [Z-INV-1]
---

# 0059 — A k9s-style workspace screen on bare `zero`

## Context

ADR 0058 gave bare `zero` a short list of workspaces. In daily use it was too thin: no view of the machine's state, no way to look into a workspace, and after an action the list was redrawn below the command's output. The user asked for a screen in the style of k9s, with the zero logo. PoC 32 showed it with bubbletea and lipgloss alone.

## Decision

1. `zero` without a command, on a terminal, opens a full-screen workspace screen (alternate screen). Without a terminal it prints the help and exits with 2, unchanged.
2. Layout: a top block (current workspace, runtime, gateway, version; the keys; the logo), a framed table of the workspaces (`NAME STATUS DIRS TOKEN POLICY`), one status line. Narrow or low windows drop the keys, then the logo, then the top block; nothing is wider than the window.
3. Modes as in k9s: `/` filter, `:` command (`q`, `help`, `all`, `new`, a workspace name), `d` describe, `?` help; `esc` leaves a mode and never quits.
4. The table loads in the background, reloads every 5 seconds and after each action, and keeps the selection by name. The screen never waits for the gateway.
5. The screen has no logic of its own: `u`, `s`, `r` run `zero -c NAME up|stop|rm` as a subprocess with the terminal handed over, so every question and warning of those commands stays (the typed name before `rm`); `enter`/`o` and `v` leave the screen and run the agent command. A failed command shows its last line in the status line.
6. Libraries: bubbletea and lipgloss (Charm v2, MIT), already in the build since ADR 0058; huh only for the new-workspace form. No table or viewport component.
7. The logo is the mark of `zero.png`, reduced to 9x6 half-block characters, with the wordmark `Z3R0`.
8. This replaces the list of ADR 0058 (its points 1 and 6 now describe this screen); the first run of ADR 0058 is unchanged.

## Consequences

- The model is a pure function of messages; its frames are tested without a terminal (sizes, filter, command, describe, keys).
- Colour comes from the terminal's 16 colours; bubbletea removes it when the terminal cannot show it or `NO_COLOR` is set.
- Each reload asks the gateway and the forge (about a second of work every 5 seconds while the screen is open).
- Not part of this: styled help and `zero ls`, the switch picker, mouse support (#181 stays open for them).
