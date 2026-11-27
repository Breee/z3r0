---
status: accepted
date: 2026-11-27
deciders: bree
requirements: [Z-OS-3]
---

# 0057 — Docker as a runtime next to podman, chosen by one setting

## Context

zero runs sandboxes with OpenShell's podman driver, rootless, under a separate user `openshell` (ADR 05). Many machines have Docker and no podman, and the opt-in parts already run under Docker (ADR 0051). OpenShell 0.1.8 has a Docker driver. PoC 29 showed a sandbox under it against a rootless and against a rootful daemon: sandbox Ready, agent uid 1000, a host outside the policy refused, both containers non-root with every capability dropped and no-new-privileges, the read-only input mount (ADR 0039) works (#167).

The two Docker modes are not equal. A rootless daemon runs as an ordinary user, like podman. The rootful daemon runs as root, and whoever can reach its socket is root on the host (ADR 05, C25).

## Decision

1. The runtime is one setting at install: `podman` (default), `docker-rootless`, `docker`. Nothing else in a workspace's config changes with it.
2. podman stays the default and the recommended choice.
3. `docker-rootless` is offered as equal in kind to podman: the daemon runs as the user that runs the gateway.
4. `docker` (the regular, rootful daemon) is offered, and zero says plainly that it may be dangerous: the user that runs the gateway gets the root daemon's socket, which equals root on the host. The warning is shown at install, as a line in `zero doctor`, and in the guide. It is a warning, not a refusal.
5. The separate user `openshell` stays the default for every runtime, as an extra layer between the login user and the sandboxes. Running the gateway as the login user is an opt-out.
6. The agent's side does not change: no runtime socket in the sandbox, agent never root (ADR 05), under every runtime.

## Consequences

- Install writes the gateway's config for the chosen driver (`compute_driver`, `[openshell.drivers.<driver>]` with the same mount settings); the input mount is passed under the driver's name.
- Under Docker there is no counterpart to podman's image allowlist (`policy.json`); zero's pinned image digest is the only check there.
- Rootless Docker before 29.10.0 cannot mount an input folder from a `nosuid`/`nodev` filesystem (C60); zero's staging folder `/var/lib/zero/input` is normally not on one.
- Shown so far: OpenShell's sandbox under both daemons with the gateway run by the login user (PoC 29). Not yet shown: the gateway under `openshell` with Docker, and zero's own flow (`zero up`, forge rule, push gate, containment tests) on Docker; both need `sudo zero install`.
- Tests and docs name the runtime they ran on.
