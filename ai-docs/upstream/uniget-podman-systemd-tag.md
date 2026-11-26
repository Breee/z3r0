# uniget podman: built without the `systemd` build tag, so container health checks never run

**Filed:** https://github.com/uniget-org/tools/issues/30167 (2026-11-26). Related: `uniget-podman-quadlet-musl.md`. Tracked in Breee/z3r0#174.

---

**Tool:** podman 6.2.0 from uniget (`tools/podman/Dockerfile.template`, line 53: `BUILDTAGS='seccomp selinux apparmor exclude_graphdriver_devicemapper containers_image_openpgp'`).

**Steps** (regular user with a systemd user session, Ubuntu 26.04):

```
go version -m /usr/local/bin/podman | grep -- -tags     # no systemd
podman run -d --name hc --health-cmd 'true' --health-interval 2s docker.io/library/alpine sleep 600
sleep 10
podman inspect --format '{{.State.Health.Status}}' hc    # starting
systemctl --user list-timers --all | grep -c "$(podman inspect --format '{{.Id}}' hc)"   # 0
podman healthcheck run hc; podman inspect --format '{{.State.Health.Status}}' hc         # healthy
```

**Expected:** the health check runs every 2 s by itself; status becomes `healthy`.

**Actual:** status stays `starting` for ever; no timer exists. With Compose, a service with `depends_on: condition: service_healthy` never starts. Also `podman info` shows cgroup manager `cgroupfs` and events `file` on a systemd host.

**Cause:** podman makes health check timers only in `libpod/healthcheck_linux.go` (`//go:build !remote && systemd`); without the tag `healthcheck_nosystemd_linux.go` does nothing and reports no error. The defaults for cgroup manager and events come from the same tag (containers/common `pkg/config/nosystemd.go`).

**Request:** build podman with the `systemd` tag (needs the systemd headers at build time, which the static Alpine build does not have), or state in the tool's notes that health checks, the systemd cgroup manager and journald events are not available.

**Versions:** podman 6.2.0 (uniget), uniget 0.30.4, docker-compose 5.3.1, Ubuntu 26.04, kernel 7.0.0-28.
