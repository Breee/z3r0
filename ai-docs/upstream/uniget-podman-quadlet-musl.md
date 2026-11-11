# uniget-org/tools: podman quadlet/rootlessport are musl-linked, not executable on glibc hosts

Existing issues: none found (searched "quadlet", "musl podman" in uniget-org/tools, 2026-11-11).

## Issue draft

**Title:** podman: quadlet and rootlessport are dynamically linked against musl, fail on glibc hosts

**Versions:** uniget tool `podman` 6.1.3 (`master` builds 6.2.0 with the same Dockerfile), host Ubuntu 26.04 (glibc, no musl loader).

**Repro**
```
$ file /usr/local/libexec/podman/quadlet
ELF 64-bit LSB executable, x86-64, dynamically linked, interpreter /lib/ld-musl-x86_64.so.1
$ /usr/local/libexec/podman/quadlet --dryrun
cannot execute: required file not found
```
`/usr/local/lib/systemd/user-generators/podman-user-generator` points to it, so systemd silently generates no units: any `~/.config/containers/systemd/*.container` → `Unit <name>.service not found`. `rootlessport` has the same problem (rootless `-p` port forwarding).

**Expected:** helpers statically linked like `podman` itself.

**Cause:** podman's Makefile links `quadlet` and `rootlessport` with `LDFLAGS_PODMAN`, but these binaries import no cgo packages, so Go uses the internal linker and ignores `-extldflags '-static'`; with `CGO_ENABLED=1` they still get the musl interpreter.

## PR draft

**Branch:** `fix-podman-static-helpers` · **Title:** podman: build quadlet and rootlessport statically

**Body:**
quadlet and rootlessport don't use cgo, so `-extldflags '-static'` is ignored and they end up linked against `/lib/ld-musl-x86_64.so.1`, which glibc distros don't have (Quadlet generator silently fails). Rebuild both with `CGO_ENABLED=0` after the main build.

Tested locally (podman build of `tools/podman`, version 6.2.0):
- `file libexec/podman/quadlet` → statically linked (before: musl interpreter)
- `file libexec/podman/rootlessport` → statically linked
- `bin/podman` unchanged, statically linked
- `quadlet --dryrun` on a sample `.container` generates the service on Ubuntu 26.04

Patch: `uniget-podman-quadlet-musl.patch` (same directory).
