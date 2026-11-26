# PoC 19 — Health checks under rootless podman: why a service waiting for `service_healthy` never starts (#169)

Question: zero's model proxy is a compose file where LiteLLM waits for a healthy Postgres. Under `podman compose` on this host it never starts (C58). Why, and what makes it work?

Result (2026-11-26, podman 6.2.0 installed with uniget, docker-compose v5.3.1 as compose provider, Docker 29.5.2 as contrast): **the installed podman never runs a health check by itself, because it was built without the `systemd` build tag.** The compose file and the cgroup manager are not the cause.

## What the sources say

| Fact | Source |
| --- | --- |
| podman runs health checks only through a transient systemd timer (`systemd-run [--user] --on-unit-inactive=<interval> … podman healthcheck run <id>`). This code is compiled only with the `systemd` build tag | [libpod/healthcheck_linux.go](https://github.com/containers/podman/blob/v6.2.0/libpod/healthcheck_linux.go) |
| Without the tag, making and starting the timer do nothing and report no error | [libpod/healthcheck_nosystemd_linux.go](https://github.com/containers/podman/blob/v6.2.0/libpod/healthcheck_nosystemd_linux.go) |
| The same tag decides the defaults seen on this host: cgroup manager `cgroupfs`, events `file` | containers/common `pkg/config/nosystemd.go` |
| containers.conf has no setting that turns the timers on | containers.conf(5) |
| Compose waits for `service_healthy` by reading the container's health status from the engine | docker/compose v5.3.1 `pkg/compose/convergence.go` |
| uniget builds podman static on Alpine with `BUILDTAGS='seccomp selinux apparmor exclude_graphdriver_devicemapper containers_image_openpgp'` | [tools/podman/Dockerfile.template](https://github.com/uniget-org/tools/blob/main/tools/podman/Dockerfile.template), line 53 |

## Result of `run.sh`

`a` has a health check every 2 s, `b` starts when `a` is healthy.

| Check | podman 6.2.0 (uniget) | Docker 29.5.2 |
| --- | --- | --- |
| Build tags hold `systemd` | no | n/a |
| Health of `a` after 30 s | `starting` | `healthy` after 5 s |
| Health check timers in the user's systemd | 0 | n/a |
| `b` started by itself | no | yes |
| After `podman healthcheck run` by hand | `a` healthy, `b` starts | n/a |

So the check itself works under podman and compose reacts to it; only the timer that should run it is missing.

## What follows

- A podman built with the `systemd` tag is needed for any compose file that waits for a healthy service. Not run here: such a build (it needs an install with sudo).
- A loop that calls `podman healthcheck run` from outside would hide the missing part; not used.
- Decided (ADR 0051): the parts run with `docker compose` until a podman with health checks is on the host; reported upstream as uniget-org/tools#30167.

## Limits

- One host. A podman with the tag was not run, so "works with the tag" rests on the source and on the check by hand.
- With the tag, the timers also need the user's systemd session (D-Bus); this host has one.

## Run

```
pocs/19-podman-healthcheck/run.sh                # podman; exit 1 when b does not start by itself
ENGINE=docker pocs/19-podman-healthcheck/run.sh
```

Needs `go` on the host to print the build tags. Cleans up with `rm -f`, not `compose down` (C57).
