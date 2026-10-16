# Phases

Follows Z3R0 arch §25. One phase = several sprints. A phase starts only after the previous phase's exit check passes.

| Phase | Arch §25 | Goal | Exit check | Status |
|---|---|---|---|---|
| 1 OpenShell + integration eval | Phase 0 items 1–4, 9 | Know every OpenShell feature we rely on; know every way an agent client (OpenCode, VS Code, others) can run on it; pick an option per client | Feature matrix + integration options matrix, each row with a PoC or doc ref; decisions in decisions.md | done 2026-10-15 (sprints 01, 02) |
| 2 Z3R0 control loop on laptop | Phase 0 items 5–8, 11, 12 + Phase 1 | Go dev loop: code → test → container build → signed commit → push feature branch → glab pipeline → MR, with brokered short-lived creds | §25 Phase 0 items 5–8, 11, 12 proven by scripts | to plan |
| 3 Kubernetes operator preview | Phase 2 + Phase 0 item 10 | DEV/QA by policy, PROD sanitized one-shot diagnostics, changes only via GitOps MR | No prod mutation path for agents | later |
| 4 Stronger isolation | Phase 3 | MicroVM profiles for untrusted repos/workloads | Containment + attack suites pass on VM driver | later |
| 5 Remote/shared execution | Phase 4 | OpenShell Kubernetes compute for teams | — | later |

## Phase 1 — exit record (2026-10-15)

- Feature matrix: `openshell-feature-matrix.md`, every row has PoC/doc ref.
- Integration matrix: `integration-options-matrix.md`; OpenCode → D08, VS Code → D09, others concept-only.
- Decisions D01–D12 in `decisions.md`.
- Sprint 03 "hardening" (F12–F16) re-slotted: input to Phase 2 planning (F16 MicroVM → Phase 4).

## Phase 2 — to plan

Planning happens after Phase 1 exit. Inputs: open attack paths (attack-paths.md), sprint 03 drafts (F12–F16), §25 items 5–8, 11, 12.
