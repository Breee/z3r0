# uniget podman: quadlet/rootlessport musl-linked, no systemd units

Target: PR https://github.com/uniget-org/tools/pull/29701 (fork Breee/uniget-tools, branch `fix-podman-static-helpers`; supersedes gitlab.com/uniget-org/tools!1). GitHub issue: https://github.com/uniget-org/tools/issues/29354. Tracking: #67, #120. Caveats C45, C53.

## Issue

**What (1):** `tools/podman` 6.1.3 and 6.2.0 ship `libexec/podman/quadlet` and `rootlessport` dynamically linked against `/lib/ld-musl-x86_64.so.1`. On glibc hosts (Ubuntu 24.04, 26.04) they cannot execute. systemd's quadlet generator fails silently: no `.service` units from `.container` files.

**Why (1):** Podman's Makefile passes `-extldflags '-static'` via `LDFLAGS_PODMAN`, but both binaries have no cgo, so Go uses the internal linker and ignores it. With `CGO_ENABLED=1` on Alpine they get the musl interpreter.

**What (2):** the package has no systemd units (`podman.socket`, `podman.service`, `podman-auto-update.*`, `podman-restart.service`, `podman-kube@.service`, …), only the two generator symlinks (`uniget inspect podman`). `systemctl --user enable --now podman.socket` → `Unit file podman.socket does not exist.`

**Why (2):** in podman's Makefile `install.systemd` is empty unless `BUILDTAGS` contains `systemd` (v6.2.0 Makefile, `ifneq (,$(findstring systemd,$(BUILDTAGS)))`). The Alpine build stage has no libsystemd, so `systemd` is not in `BUILDTAGS` and `make install.systemd` installs nothing.

**Repro:** `uniget install podman` → `file /usr/local/libexec/podman/quadlet` → `interpreter /lib/ld-musl-x86_64.so.1`; `find /usr/local/lib/systemd -name 'podman*'` → only the generators.

## Comment for #29354 / MR description addition

> Second problem in the same package: no systemd units. `install.systemd` in podman's Makefile is empty unless `BUILDTAGS` contains `systemd`, which the Alpine build cannot use (no libsystemd). The units do not depend on the tag, so the MR now runs `make install.systemd … BUILDTAGS=systemd BINDIR=/usr/local/bin` for the install step only. `BINDIR` makes `ExecStart` point to `/usr/local/bin/podman` instead of `/bin/podman` (`PREFIX=""`).

## MR

**Title:** fix(podman): build quadlet and rootlessport statically, ship systemd units

**What:**
- Rebuild `bin/quadlet` and `bin/rootlessport` with `CGO_ENABLED=0` after the main build (patch: `uniget-podman-quadlet-musl.patch`).
- `make install.systemd` with `BUILDTAGS=systemd BINDIR=/usr/local/bin`.

**How tested:** see MR (`make podman`, `make podman--tar`, `uniget install --path-to-tar-mappings=podman=tools/podman/image.tar`).
