---
status: accepted
date: 2026-11-11
deciders: bree
requirements: [Z-INV-5]
---
# 0028 — Dev image: CI-signed, pinned by digest, no host signature check yet

## Context
D21: dev image from ghcr.io, signed, pulled by digest. podman policy cannot verify GitHub Actions keyless signatures (C47).

## Decision
CI builds and cosign-signs `ghcr.io/breee/z3r0-dev` keyless. zero pins the digest in source (`devImage`); digest bumps go through review. podman policy allows only that repository. No host-side signature check for now.

## Consequences
- Integrity from the digest; build origin is checkable manually with `cosign verify`.
- Revisit (host or admission check) when digest bumps are automated, contributors grow, or K8s phase; or when C47 is fixed upstream.
