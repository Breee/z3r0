---
status: accepted
date: 2026-10-16
deciders: bree
requirements: [Z-OS-7]
---

# 15. One driver per OpenShell gateway, so zero runs two gateways (`openshell` = podman/E1, `openshell-vm` = vm/E3) and selects by risk class via `zero up --vm` / `Z3R0_DRIVER` / repo `.z3r0.yaml` (arch §6.2 C0–C4→E1/E3)

## Context and Problem Statement

0.1.2 `compute_driver` is singular and `sandbox create` has no driver flag; separate gateways also separate policy, credstore and host user.

## Decision Outcome

One driver per OpenShell gateway, so zero runs two gateways (`openshell` = podman/E1, `openshell-vm` = vm/E3) and selects by risk class via `zero up --vm` / `Z3R0_DRIVER` / repo `.z3r0.yaml` (arch §6.2 C0–C4→E1/E3).
