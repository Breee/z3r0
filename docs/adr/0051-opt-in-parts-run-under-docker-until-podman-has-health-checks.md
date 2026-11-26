---
status: accepted
date: 2026-11-26
deciders: bree
requirements: [Z-INV-1, Z-INV-5]
---

# 0051 — The opt-in parts run under Docker until the podman here runs health checks

## Context

The model proxy starts from a compose file in which LiteLLM waits for its database to be healthy. The podman installed with uniget is built without the `systemd` build tag, so it never runs a container health check by itself (a check run by hand works) and the proxy never starts (C58, PoC 19, sources in #169). No setting changes this. Reported upstream: uniget-org/tools#30167 (#174).

## Decision

1. The opt-in parts (model proxy and its database today) are started with `docker compose`. `ZERO_COMPOSE` names another compose command.
2. This holds until a podman with health checks is on the host (upstream fix or another podman source); then PoC 19 is run under podman and the default goes back to `podman compose`.
3. Sandboxes are not touched: they stay under the rootless podman of the dedicated gateway user (ADR 05).
4. The health-check dependency stays in the compose file; it is not dropped to make podman pass.

## Consequences

- A workspace that uses the model proxy needs Docker and a user who may use it. On this host that is the rootful daemon and the `docker` group (C25): the user's own account can reach root through it. The agent has no path to the Docker socket: its sandbox is a container of the dedicated gateway user's rootless podman (`internal/zero/install.sh`), the socket is not mounted into it and `/run` is not in the sandbox's file policy (`internal/zero/policy.yaml`). Not tested by an attack run.
- The parts hold credentials (provider logins, proxy master key) in containers of a root daemon; ports stay on 127.0.0.1.
- Accepted risk for this time: a compromised part image (LiteLLM had two compromised releases in 2026) runs under a root daemon, which is weaker containment than rootless podman (Z-INV-5 holds for the sandbox, only partly for the parts). Kept small by: images pinned by digest, no host folder mounted except the rendered config, no privileged flag, loopback ports.
- Without Docker, `zero up` of such a workspace fails with the compose command's error; workspaces without `parts` are not affected.
- #174 stays open until point 2 is done.
