---
status: accepted
date: 2026-11-16
deciders: bree
requirements: [Z-INV-4, Z-INV-6]
---
# 0031 — zero serve: one host process for push gate and token upkeep

## Context
The push gate (ADR 0030) ran as its own detached process, and forge tokens were renewed and orphans revoked only when someone ran `zero up`. A sandbox left running over night kept an expiring token; a crashed sandbox kept a live one.

## Decision
- `zero serve` is the only long-running zero process on the host. It hosts the push gate (127.0.0.1:18095) and runs an upkeep loop at start and every 10 minutes: renew the token of every live forge sandbox that expires within 12 h, revoke this machine's zero tokens that no live sandbox owns.
- `zero up` starts it if it is not answering; `sudo zero install` installs `/usr/local/bin/zero` and a systemd user unit `zero.service` for the login user; `zero doctor` checks it.
- It runs as the login user and uses that user's forge login (glab). It holds no credential of its own.
- Upkeep takes the per-sandbox lock; a sandbox another zero command is working on is skipped. Capabilities for re-minting come from the state file `caps` written by `zero up`, not from the checkout.
- `zero up` and `zero rm` still mint and revoke in the CLI.
- The PoC programs moved out of `cmd/` to `pocs/09-zero-cp/zero-cp` and `pocs/11-push-service/zero-push`.

## Consequences
- Renewal and orphan cleanup work while the laptop is on and the user logged in. Revocation still depends on the laptop (Z-INV-6 not met until a central deployment, #92).
- Running processes keep the old token after renewal (C44).
- A policy change in `.zero/policy.yaml` reaches renewal only after the next `zero up`.
- One log: `journalctl --user -u zero` (or `~/.local/state/zero/serve.out` without the unit); gate decisions stay in `<state>/gate.log` too.
- Hardening (#100): the gate refuses pushes with more than 32 ref updates or a command line over 1024 bytes and cuts a command list that does not arrive within 30 s; the parser has a fuzz test (`FuzzGateCommands`). The unit runs with NoNewPrivileges, PrivateTmp, ProtectSystem=strict, ProtectHome=read-only (writable: zero state dir, glab config), ProtectKernelTunables, ProtectControlGroups, RestrictSUIDSGID, RestrictNamespaces, RestrictAddressFamilies, LockPersonality, MemoryDenyWriteExecute, SystemCallArchitectures=native (`systemd-analyze --user security`: 6.7 MEDIUM). `ProtectKernelModules` cannot be used in a user unit (needs to drop capabilities) and is left out. Without the unit (detached `zero serve`) none of the unit hardening applies.
