# uniget podman: quadlet/rootlessport musl-linked

Target: MR to gitlab.com/uniget-org/tools (issues disabled there; GitHub mirror issue optional). Tracking: #67.

## Issue

**What:** `tools/podman` 6.1.3 ships `libexec/podman/quadlet` and `rootlessport` dynamically linked against `/lib/ld-musl-x86_64.so.1`. On glibc hosts (Ubuntu 26.04) they cannot execute. systemd's quadlet generator fails silently: no `.service` units from `.container` files.

**Why:** Podman's Makefile passes `-extldflags '-static'` via `LDFLAGS_PODMAN`, but both binaries have no cgo, so Go uses the internal linker and ignores it. With `CGO_ENABLED=1` on Alpine they get the musl interpreter.

**Repro:** `uniget install podman` → `file /usr/local/libexec/podman/quadlet` → `interpreter /lib/ld-musl-x86_64.so.1`; put a `.container` in `~/.config/containers/systemd/`, `systemctl --user daemon-reload` → unit not found.

## MR

**Title:** podman: build quadlet and rootlessport statically

**What:** Rebuild `bin/quadlet` and `bin/rootlessport` with `CGO_ENABLED=0` after the main build (patch: `uniget-podman-quadlet-musl.patch`).

**Why:** Binaries are musl-linked and do not run on glibc hosts; quadlet generator silently produces no units.

**How tested:** `make podman--tar`, `uniget install --path-to-tar-mappings=podman=.../image.tar` on Ubuntu 26.04: both binaries `statically linked`; `.container` file → `systemctl --user daemon-reload` → generated `zq.service` present.
