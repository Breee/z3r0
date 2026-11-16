---
status: accepted
date: 2026-11-16
deciders: bree
requirements: [Z-INV-4]
---
# 0037 — opencode pairing link instead of a serve password

## Context

zero started `opencode serve` in the sandbox with a random password and kept that password in plain text in its state dir (#89). `zero web` printed it and the developer typed it into the browser.

opencode 2.2.6 has a background service (`opencode service …`) and `opencode pair`, which prints links that work once and expire after 5 minutes.

## Decision

- zero runs the opencode background service in the sandbox (`opencode service set hostname 127.0.0.1`, `set port 4096`, `service start`) instead of `opencode serve` with a password.
- `zero web` asks the sandbox for a pairing link (`opencode pair --url http://127.0.0.1:<forwarded port>`) and opens it. No password exists; the old `password` state file is deleted on the next `zero up`.
- `opencode pair --remote` (public tunnel) is never used.
- `zero logs` shows the service log (`~/.local/share/opencode/log/opencode.log`).

## Consequences

- No long-lived secret for the web UI in zero's state. The link is in the terminal output and the browser history, but it is dead after one use or 5 minutes.
- The service is reachable only through the SSH tunnel on 127.0.0.1; without pairing the API answers 401.
- Not known: what the paired browser stores and how a pairing is revoked other than recreating the sandbox.
- `zero attach` and `zero run` send no credential and stay unusable (C38, #55).
- The agent runs in the same sandbox as the service and can create pairing links itself; they are only usable by someone who can reach the tunnel port on the developer's machine (P22).
