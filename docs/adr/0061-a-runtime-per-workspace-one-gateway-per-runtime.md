---
status: proposed
date: 2026-11-28
deciders: bree
requirements: [Z-OS-3]
---

# 0061 — A runtime per workspace; one gateway per runtime

## Context

ADR 0057 made the runtime one setting of the install: `podman`,
`docker-rootless` or `docker`. Changing it means removing every workspace
and installing again.

The user wants to choose per workspace (#180, 2026-11-27):
`zero opencode --podman`, `zero opencode --rootless-docker`, and later
other sandbox systems such as nono.

What was shown:

- PoC 34: OpenShell 0.1.8 does not start a sandbox under gVisor. Its podman
  and Docker drivers have no setting for the OCI runtime, so the runtime is
  the engine's default; with `runsc` as that default the sandbox's
  capability check fails (caveat C64).
- PoC 35: two OpenShell gateways, one with the podman driver and one with
  the Docker driver, run side by side. Each holds its own sandboxes, and
  `openshell --gateway NAME` picks one. zero already runs two gateways this
  way (`openshell` and `openshell-vm`).

## Decision

1. A runtime is one named definition: the engine, the OCI runtime that
   engine uses by default, and one gateway (name, port, unit, config), with
   what it needs on the host and its risk note. Adding a runtime adds a
   definition and does not touch the others.
2. `zero install` sets up one gateway for every runtime whose needs are met
   on the host. The gateways run side by side under the user `openshell`.
   Rootful Docker is set up only when asked for, with the warning of
   ADR 0057.
3. A workspace has a runtime: `runtime:` in its config, or a flag at the
   first `zero up` (`--podman`, `--docker-rootless`, `--docker`). Without
   either it is podman. A sandbox keeps its runtime until `zero rm`.
4. zero picks the gateway by the workspace's runtime. Everything else in
   the flow stays the same.
5. `zero doctor` prints one line per installed runtime. The warning for
   rootful Docker stays.
6. gVisor is not offered until OpenShell can run under it (C64).
7. Another sandbox system (nono) is not a runtime. It is a
   `SandboxProvider` (ADR 0060) and gets its own research and PoC.
8. Once built, this replaces the install-wide switch of ADR 0057. The
   other points of ADR 0057 stay: the `openshell` user, the danger warning,
   no `PrivateTmp` under rootless Docker.

## Consequences

- More than one gateway process runs on the host, each on its own fixed
  port.
- Caveat C61 applies to the rootless Docker gateway and C62 to every
  gateway: a gateway stopped during a sandbox start can leave that sandbox
  locked.
- The image allowlist exists only under podman (ADR 0057).
- Not shown yet: the gateways as installed units under `openshell`, and
  zero's own flow (`zero up`, push gate, attack suite) on a second gateway.
  Both need `sudo zero install` by the user.
