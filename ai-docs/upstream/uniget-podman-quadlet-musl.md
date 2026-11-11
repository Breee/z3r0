# uniget-org/tools: podman quadlet generator is musl-linked, not executable on glibc hosts

Existing issues: none found (searched "quadlet", "musl podman" in uniget-org/tools, 2026-11-11).

## Versions
- uniget tool `podman` 6.1.3 (installed under /usr/local)
- Host: Ubuntu 26.04 LTS (glibc, no musl loader)

## Repro
```
$ file /usr/local/libexec/podman/quadlet
ELF 64-bit LSB executable, x86-64, dynamically linked, interpreter /lib/ld-musl-x86_64.so.1
$ /usr/local/libexec/podman/quadlet --user --dryrun
cannot execute: required file not found
```
`/usr/local/lib/systemd/user-generators/podman-user-generator` symlinks to it, so systemd silently generates no units:
any `~/.config/containers/systemd/*.container` → `Unit <name>.service not found`.

## Expected
quadlet statically linked (like `podman` itself) or built for glibc, so Quadlet units work on glibc distros.

## Actual
Generator fails to exec; Quadlet is unusable.
