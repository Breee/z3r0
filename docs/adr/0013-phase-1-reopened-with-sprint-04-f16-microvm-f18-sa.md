---
status: accepted
date: 2026-10-16
deciders: bree
requirements: [Z-P0-8, Z-P0-9, Z-OS-7]
---

# 13. Phase 1 reopened with sprint 04 (F16 MicroVM, F18 sandbox build) before Phase 2 planning

## Context and Problem Statement

arch §9.5 requires in-sandbox container build and routes hostile Dockerfiles to E3; neither OpenShell assumption is tested, and E1 blocks userns so Dockerfile builds likely need the VM driver.

## Decision Outcome

Phase 1 reopened with sprint 04 (F16 MicroVM, F18 sandbox build) before Phase 2 planning.
