# Phases

Follows Z3R0 arch §25. One phase = several sprints. A phase starts only after the previous phase's exit check passes.

| Phase | Arch §25 | Goal | Exit check | Status |
|---|---|---|---|---|
| 1 OpenShell + integration eval | Phase 0 items 1–4, 9 | Know every OpenShell feature we rely on; know every way an agent client (OpenCode, VS Code, others) can run on it; pick an option per client | Feature matrix + integration options matrix, each row with a PoC or doc ref; decisions in decisions.md | reopened 2026-10-16: sprint 04 (F16 MicroVM, F18 sandbox build) |
| 2 Z3R0 control loop on laptop | Phase 0 items 5–8, 11, 12 + Phase 1 | Go dev loop: code → test → container build → signed commit → push feature branch → glab pipeline → MR, with brokered short-lived creds | §25 Phase 0 items 5–8, 11, 12 proven by scripts | planned: sprints 05, 06 |
| 3 Kubernetes operator preview | Phase 2 + Phase 0 item 10 | DEV/QA by policy, PROD sanitized one-shot diagnostics, changes only via GitOps MR | No prod mutation path for agents | later |
| 4 Stronger isolation | Phase 3 | MicroVM profiles for untrusted repos/workloads | Containment + attack suites pass on VM driver | later |
| 5 Remote/shared execution | Phase 4 | OpenShell Kubernetes compute for teams | — | later |

## Phase 1 — exit record (2026-10-15, reopened 2026-10-16)

- Feature matrix: `openshell-feature-matrix.md`, every row has PoC/doc ref.
- Integration matrix: `integration-options-matrix.md`; OpenCode → D08, VS Code → D09, others concept-only.
- Decisions D01–D12 in `decisions.md`.
- Sprint 03 "hardening" (F12–F15) re-slotted: input to Phase 2 planning.
- Reopened: Appendix A gaps MicroVM driver + in-sandbox container build block Phase 2 (`openshell-feature-matrix.md` gap evaluation) → sprint 04 F16, F18. Agent summary of the arch: `arch-caveman.md`.

## Phase 2 — plan (2026-11-10)

Goal: one Go change goes code → test → build → signed commit → push → pipeline → MR from a zero sandbox, with short-lived creds, an audit trail and revocation.

| §25 item | Today | Phase 2 work | Sprint |
|---|---|---|---|
| Phase 1 loop | manual steps work (`docs/scenarios/go-container-gitlab.md`) | scripted end-to-end check | 06 |
| 5 OpenBao GitLab creds | zero mints PAT via glab | OpenBao issues/revokes the token, rotation without stale placeholders (C44) | 05 |
| 6 branch push / protected deny | done (F12, F14); any-branch risk C43 | — (GitLab-owned) | — |
| 7 glab pipeline | done | part of loop check | 06 |
| 8 build w/o docker socket | ko image build in sandbox | part of loop check; Dockerfile builds deferred to Phase 4 | 06 |
| 11 trace | none | per-sandbox audit log from OpenShell events + zero actions | 05 |
| 12 revoke | token revoked on `zero rm` | `zero revoke` soft/hard/quarantine, leftover-token cleanup after crash | 05 |
| signed commit (Phase 1 loop) | unsigned | per-session signing, verified on GitLab | 06 |

Sprint 05 — zero core: audit log, revocation, OpenBao broker, signed ghcr image, policy widening prompt. Client-independent.
Sprint 06 — control loop: signed commits, end-to-end loop script on top of the core.

Exit check: `pocs/07-control-loop/run.sh` passes end to end; audit log shows every step; `zero revoke` stops push within seconds.
