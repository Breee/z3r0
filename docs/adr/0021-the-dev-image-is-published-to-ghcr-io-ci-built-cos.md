---
status: accepted
date: 2026-11-10
deciders: bree
requirements: []
---

# 21. The dev image is published to ghcr.io (CI-built, cosign-signed, digest-pinned) and pulled from there

## Context and Problem Statement

user decision; reproducible image, no local sudo build.

## Decision Outcome

The dev image is published to ghcr.io (CI-built, cosign-signed, digest-pinned) and pulled from there; the local `scripts/build-dev-image.sh` build is interim only. Until then the gateway policy allowlists ghcr.io/uniget-org (base tools pinned by digest).
