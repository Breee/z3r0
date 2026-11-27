# PoC 29 — a sandbox under Docker, rootless and rootful (#167)

Question: does OpenShell run a sandbox with the same limits when its runtime is Docker instead of podman, against a rootless and against a rootful daemon?

Result (2026-11-27, OpenShell 0.1.8, Docker 29.5.2): **yes for both.** `run.sh`: 15 PASS, 0 FAIL against each daemon.

## Sources

- OpenShell v0.1.8 `docs/how-it-works/sandboxes/runtimes.mdx` (drivers; rootless is named for Podman only)
- OpenShell v0.1.8 `crates/openshell-driver-docker/README.md` (`socket_path`)
- `openshell-gateway --help` (`--config`, `--port`, `OPENSHELL_*` environment)
- https://docs.docker.com/engine/security/rootless/ (`dockerd-rootless-setuptool.sh install`)
- OpenShell v0.1.8 `docs/how-it-works/sandboxes/runtimes.mdx`, driver config mounts (`{"docker":{"mounts":[...]}}`, `allow_driver_config`, `enable_bind_mounts`, resource admission)
- https://docs.docker.com/engine/release-notes/29/ (29.10.0, moby/moby#53981)
- https://docs.docker.com/engine/security/#docker-daemon-attack-surface (the rootful socket equals root on the host)

## Setup

```
run.sh -> throwaway gateway as the calling user, port 17690,
          compute_driver = "docker", [openshell.drivers.docker] socket_path = SOCKET,
          bind mounts on (allow_driver_config, enable_bind_mounts, resource admission off)
       -> one sandbox from zero's dev image with zero's policy and a read-only
          input folder at /tmp/zero-input (as zero mounts it, ADR 0039) -> checks -> everything removed
```

## Result

| Check | Rootless (`$XDG_RUNTIME_DIR/docker.sock`) | Rootful (`/var/run/docker.sock`) |
|---|---|---|
| Daemon security options | seccomp, rootless, cgroupns | apparmor, seccomp, cgroupns |
| Gateway uses the Docker driver | PASS | PASS |
| Sandbox ready | PASS | PASS |
| Agent is not root (`uid=1000`) | PASS | PASS |
| Host outside the policy refused (`000`) | PASS | PASS |
| Input folder readable in the sandbox | PASS | PASS |
| Input folder not writable from the sandbox | PASS | PASS |
| Two containers at this daemon only | PASS | PASS |
| Supervisor: user `65534:65534`, not privileged, all capabilities dropped, no-new-privileges | PASS | PASS |
| Workload: user `1000:1000`, not privileged, all capabilities dropped, no-new-privileges | PASS | PASS |

## Findings

- The Docker driver works against a rootless daemon, although OpenShell's docs name rootless only for Podman.
- The containers look the same under both daemons. The difference is the daemon: rootless runs as the user, rootful runs as root.
- **Rootful Docker is dangerous**: whoever can use `/var/run/docker.sock` is root on the host (ADR 05, C25). The gateway's user needs that socket, so a hole in the gateway or a container escape ends at root, not at an unprivileged user.
- The input mount works with the same three settings as under podman; the block in `--driver-config-json` is named `docker` instead of `podman`.
- Rootless Docker 29.5.2: a read-only bind mount whose source is on a `nosuid`/`nodev` filesystem (here `/tmp`, a tmpfs) makes the sandbox fail: `BoundaryConfigWriteFailed ... remount-ro ... flags: 0x1021: operation not permitted`. Fixed in Docker 29.10.0 (moby/moby#53981); C60. zero's input folder is `/var/lib/zero/input`; on this host that is not such a filesystem. Repeat: `INPUT_PARENT=/tmp pocs/29-docker-runtime/run.sh`.
- `openshell gateway add` makes the new gateway the active one and writes its own certificates; `run.sh` selects `openshell` again and copies its certificates afterwards.
- The first `sandbox create` right after a gateway start can fail with `transport error`; `run.sh` retries.

## Limits

- The gateway ran as the login user, not as the `openshell` user. Docker for the `openshell` user (its own rootless daemon, or membership in the `docker` group) was not set up.
- podman's image allowlist (`policy.json`, only zero's images may be pulled) has no counterpart here.
- zero's own flow was not run on Docker: `zero up`, forge rule, push gate, `tests/containment`.
- Credential placeholders were not checked. No breakout test.

## Run

```
pocs/29-docker-runtime/run.sh                               # rootless
SOCKET=/var/run/docker.sock pocs/29-docker-runtime/run.sh   # rootful
```

Needs OpenShell 0.1.8 as installed by zero, a Docker daemon at `SOCKET`, port 17690 free.
