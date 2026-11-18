---
status: accepted
date: 2026-11-18
deciders: bree
requirements: [Z-INV-4]
---
# 0042 — Set setuid on uniget's newuidmap/newgidmap by hand until uniget is fixed

## Context
Rootless podman for the gateway user needs `newuidmap` and `newgidmap` setuid root. The uniget `uidmap` image has the bit, but the uniget CLI (v0.30.1–v0.30.4) drops it on install (C54, #121, upstream draft `ai-docs/upstream/uniget-cli-setuid.md`, #122). Without it, every rootless podman command fails (`newuidmap: write to uid_map failed: Operation not permitted`) and `zero install` cannot finish.

## Decision
- After `uniget install uidmap`, the admin runs `sudo chmod u+s /usr/local/bin/newuidmap /usr/local/bin/newgidmap`.
- zero does not do this itself: `zero install` keeps checking only that the binaries exist.
- The bug stays open upstream. When uniget keeps the bit, this ADR is superseded and the step is removed from the docs.

## Consequences
- The result is the mode the package intends and that distribution packages ship (`-rwsr-xr-x`, owner root). No privilege beyond that.
- A `uniget install --reinstall uidmap` or update drops the bit again; rootless podman then fails until the step is repeated.
- Manual step on every host installed with uniget.
