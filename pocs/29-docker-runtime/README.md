# PoC 29 — a sandbox under Docker, rootless and rootful (#167)

Question: does OpenShell run a sandbox with the same limits when its runtime is Docker instead of podman, against a rootless and against a rootful daemon?

Result (2026-11-27, OpenShell 0.1.8, Docker 29.5.2): **yes for both.** `run.sh`: 14 PASS, 0 FAIL against each daemon.

## Sources

- OpenShell v0.1.8 `docs/how-it-works/sandboxes/runtimes.mdx` (drivers; rootless is named for Podman only)
- OpenShell v0.1.8 `crates/openshell-driver-docker/README.md` (`socket_path`)
- `openshell-gateway --help` (`--config`, `--port`, `OPENSHELL_*` environment)
- https://docs.docker.com/engine/security/rootless/ (`dockerd-rootless-setuptool.sh install`)
- https://docs.docker.com/engine/security/#docker-daemon-attack-surface (the rootful socket equals root on the host)

## Setup

```
run.sh -> throwaway gateway as the calling user, port 17690,
          compute_driver = "docker", [openshell.drivers.docker] socket_path = SOCKET
       -> one sandbox from zero's dev image with zero's policy -> checks -> everything removed
```

## Result

| Check | Rootless (`$XDG_RUNTIME_DIR/docker.sock`) | Rootful (`/var/run/docker.sock`) |
|---|---|---|
| Daemon security options | seccomp, rootless, cgroupns | apparmor, seccomp, cgroupns |
| Gateway uses the Docker driver | PASS | PASS |
| Sandbox ready | PASS | PASS |
| Agent is not root (`uid=1000`) | PASS | PASS |
| Host outside the policy refused (`000`) | PASS | PASS |
| Two containers at this daemon only | PASS | PASS |
| Supervisor: user `65534:65534`, not privileged, all capabilities dropped, no-new-privileges | PASS | PASS |
| Workload: user `1000:1000`, not privileged, all capabilities dropped, no-new-privileges | PASS | PASS |

## Findings

- The Docker driver works against a rootless daemon, although OpenShell's docs name rootless only for Podman.
- The containers look the same under both daemons. The difference is the daemon: rootless runs as the user, rootful runs as root.
- **Rootful Docker is dangerous**: whoever can use `/var/run/docker.sock` is root on the host (ADR 05, C25). The gateway's user needs that socket, so a hole in the gateway or a container escape ends at root, not at an unprivileged user.
- `openshell gateway add` makes the new gateway the active one and writes its own certificates; `run.sh` selects `openshell` again and copies its certificates afterwards.
- The first `sandbox create` right after a gateway start can fail with `transport error`; `run.sh` retries.

## Limits

- The gateway ran as the login user, not as the `openshell` user.
- zero's own flow was not run on Docker: `zero up`, forge rule, push gate, input mount (a bind mount; needs `allow_driver_config`, `enable_bind_mounts`, resource admission off), `tests/containment`.
- Credential placeholders were not checked. No breakout test.

## Run

```
pocs/29-docker-runtime/run.sh                               # rootless
SOCKET=/var/run/docker.sock pocs/29-docker-runtime/run.sh   # rootful
```

Needs OpenShell 0.1.8 as installed by zero, a Docker daemon at `SOCKET`, port 17690 free.
