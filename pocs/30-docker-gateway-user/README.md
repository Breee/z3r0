# PoC 30: gateway under the `openshell` user with Docker (#167)

## Question

PoC 29 ran the gateway as the login user. ADR 0057 keeps the separate
`openshell` user as the default for every runtime. Does a sandbox work when
the gateway runs as `openshell` and uses Docker,

- rootless: a Docker daemon that belongs to `openshell`,
- rootful: the system daemon, reached through group `docker`,

with the input folder staged where zero stages it (`/var/lib/zero/input`)?

## Result

Yes for both (2026-11-27, OpenShell 0.1.8, Docker 29.5.2, run by the user with sudo).

| | rootless (daemon of `openshell`) | rootful (system daemon) |
|---|---|---|
| checks | 22 PASS, `fails: 0` | 19 PASS, `fails: 0` |
| gateway user, groups | `openshell`, 1001 | `openshell`, 992 (`docker`) + 1001 |
| sandbox ready, agent uid 1000 | yes | yes |
| host outside the policy | refused (000) | refused (000) |
| input folder from `/var/lib/zero/input` | readable, not writable | readable, not writable |
| containers | unprivileged, all capabilities dropped, no-new-privileges | same |
| container processes on the host | run as `openshell` | uid 65534 (supervisor), 1000 (workload) |

Findings:

- A rootless Docker daemon runs for the `nologin` user `openshell` with its
  existing subuid range and linger; `dockerd-rootless.sh` needs `/usr/sbin`
  on its PATH (`sysctl`).
- Rootful: the gateway reaches the system daemon only with group `docker`.
  That group equals root on the host (ADR 05, C25); `zero doctor` warns
  when the gateway process holds it.
- Rootful: the workload's host uid is 1000, on most machines the login
  user's uid. Rootless maps it into the subuid range of `openshell`.

## Sources

- PoC 29 (`../29-docker-runtime/`), ADR 0057.
- `/usr/local/bin/dockerd-rootless.sh` header (arguments go to `dockerd`,
  `DOCKERD_ROOTLESS_ROOTLESSKIT_STATE_DIR`), https://docs.docker.com/go/rootless/
- `setpriv(1)`: `--reuid`, `--regid`, `--groups`.

## Setup

- rootless: `dockerd-rootless.sh` started as `openshell` with its own socket
  (`/run/user/<uid>/zero-poc30/docker.sock`) and data root (a temp folder in
  the home of `openshell`). Not installed as a unit.
- rootful: the gateway process gets group `docker` for this one process
  (`setpriv --groups`); the user `openshell` is not added to the group.
- Gateway: as in PoC 29, on port 17690, run as `openshell`. The login user
  is the client.

## Checks

Those of PoC 29, plus: the daemon is rootless and its socket belongs to
`openshell`; the gateway runs as `openshell`; its groups (rootful: holds
`docker`, which is what `zero doctor` warns about; rootless: does not);
the container processes on the host are not root.

## Limits

- zero's own flow (`zero up`, forge rule, push gate, `tests/containment`) is not run.
- No image allowlist under Docker (ADR 0057).

## Run

```sh
sudo pocs/30-docker-gateway-user/run.sh               # rootless, pulls the image (~1.5 min)
sudo MODE=rootful pocs/30-docker-gateway-user/run.sh
```

One at a time. Ends with `fails: 0` when every check passes.
