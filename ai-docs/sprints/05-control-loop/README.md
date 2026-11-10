# Sprint 05 — control loop (Phase 2)

Goal: one Go change goes code → test → build → signed commit → push → pipeline → MR from a zero sandbox.
Exit: `pocs/07-control-loop/run.sh` passes on the developer project.

| ID | Feature | Arch | Status |
|---|---|---|---|
| F19 | Signed dev image on ghcr, pulled by digest | ADR 0021, §9 | todo |
| F20 | Policy widening prompt (`.zero/policy.yaml`) | §7, 07a | todo |
| F21 | Signed commits from the sandbox | §22 git signing | todo |
| F22 | End-to-end loop script (test, ko build, push, pipeline, MR) | §25 Phase 1, items 7–8 | todo |

Order: F19 → F20 → F21 → F22.
