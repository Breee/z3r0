---
status: accepted
date: 2026-11-10
deciders: bree
requirements: [Z-INV-1, Z-OS-7]
---
# 0024 — Damage minimization on a DevOps/GitOps backbone; sandbox runtime is a swappable dependency

## Context
zero must not reinvent what forges, CI and GitOps already do well. Other sandbox runtimes (e.g. Agent Substrate for K8s) may replace or complement OpenShell later.

## Decision
- zero's aim: minimize the damage an agent can do. Change reaches the world only through the normal DevOps path: branch → MR → CI → review → merge → GitOps deploy. zero adds isolation, scoped short-lived credentials and evidence around it; it does not build its own review, CI or deploy.
- The sandbox runtime is a dependency behind `internal/zero.Runtime`. OpenShell is the only implementation for now. Runtime-specific code lives only in `internal/zero/openshell.go`.

## Consequences
- New capabilities are first sought in forge/CI features (protected branches, required pipelines, approvals) before custom code.
- A second runtime (Agent Substrate) needs only a new `Runtime` implementation plus a green attack suite.
