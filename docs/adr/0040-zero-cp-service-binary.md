---
status: accepted
date: 2026-11-18
deciders: bree
requirements: [Z-INV-4, Z-INV-6]
---
# 0040 — The service is the binary zero-cp, not the command zero serve

## Context
ADR 0031 made the push gate and the token upkeep one long-running process and exposed it as the CLI subcommand `zero serve`. That reads like something a developer types, sits next to `opencode serve` (which runs in the sandbox and is what `zero status` calls "serve"), and hides that it is the control plane ADR 0025 already named `zero-cp`.

## Decision
- The service is its own binary `zero-cp` (`cmd/zero-cp`), without arguments or subcommands. The `zero` CLI has no `serve` command.
- `zero install` installs `/usr/local/bin/zero-cp` and the user unit `zero-cp.service` (same hardening as before) and removes `zero.service`.
- `zero up` still starts it when it is not answering: the unit if installed, else detached from the `zero-cp` next to `zero` or on PATH (log `~/.local/state/zero/zero-cp.out`).
- Both binaries are thin mains over `internal/zero`. Everything else in ADR 0031 stands.

## Consequences
- One name for the service in code, unit, messages and docs; the same name on the laptop and for a later central deployment.
- Two files to ship instead of one. `zero` without `zero-cp` next to it or on PATH cannot start the push gate and says so.
- A `zero serve` still running from an older install holds the port until `sudo zero install` (or `systemctl --user stop zero`).
- `zero-cp` on the laptop is far from the control plane of §27: no sessions, no token endpoint, no decision log yet.
