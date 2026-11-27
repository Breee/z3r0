---
status: accepted
date: 2026-11-28
deciders: bree
requirements: [Z-INV-1]
---

# 0058 — Guided first run and a workspace list, built with Charm

## Context

A new repo started with the default policy and no hint that a policy file exists; a person learned about `.zero/policy.yaml` from the guide or from a refused download. `zero` alone printed help. The user asked for the best everyday experience: `zero` opens a list to pick from, the agent commands start at once, and a repo without config is guided, with permissions asked and reasonable defaults offered (#185).

PoC 31 showed the questions and the list with Charm v2 (bubbletea, huh, lipgloss): defaults by Enter, the forms on stderr, no questions without a terminal.

## Decision

1. `zero` without a command opens the workspace list on a terminal; without a terminal it prints help and exits 2, as before.
2. The agent is the verb: `zero opencode`, `zero vscode`, `zero claude`, `zero copilot`. `zero up` prepares only. There is no `zero up --opencode`: one spelling per action.
3. First run (new sandbox, a repo directory without `.zero/policy.yaml`, a terminal, no `--approve`): zero asks for hosts (presets detected from the repo's files) and permissions (the default set on; `ci.run` and `gitlab.mr.create` off and explained), shows the resulting policy and writes the file on yes. That yes is the approval of the workspace policy shown, and of nothing beyond it: anything wider that a directory's own file brings along still asks.
4. Without a terminal or with `--approve` nothing is asked and nothing is written.
5. The repo owns `.zero/policy.yaml` (committed); the person owns `~/.config/zero/config.yaml`. The repo file stays untrusted input: a later widening needs approval on the host.
6. The list only runs the commands a person would type (`zero -c NAME ...`).
7. Charm v2 (`charm.land/bubbletea/v2`, `huh/v2`, `lipgloss/v2`) is the library for interactive screens. The plain paths (pipes, `NO_COLOR`, scripts) do not use it.

## Consequences

- zero's Go module gets its first dependencies besides yaml: three direct, about 25 indirect; the binary grows by a few MB. They run on the host only, never in the sandbox, and handle no credentials.
- The first-run answers are host-side input by the person at the terminal; the agent cannot give them (no sandbox exists yet).
- Presets are a short fixed table in zero; a host outside it is added by editing the file.
- Styling the other commands' output with the same library is #181; the runtime per workspace is #180.
