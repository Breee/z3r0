---
status: accepted
date: 2026-10-09
deciders: bree
requirements: []
---

# 06. Sandbox breakout = full compromise of the gateway's host user (gateway, KEK + provider secrets, podman, ~/.ssh)

## Context and Problem Statement

F03 finding, arch §14.14 "compromised workstation". Verified in F09.

## Decision Outcome

Sandbox breakout = full compromise of the gateway's host user (gateway, KEK + provider secrets, podman, ~/.ssh). Treat laptop as untrusted: run gateway+podman as a dedicated host user, only short-lived/low-value creds on laptop, authority stays server-side (MR/CI/GitOps).
