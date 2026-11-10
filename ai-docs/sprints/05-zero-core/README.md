# Sprint 05 — zero core (Phase 2)

Goal: zero works on its own, independent of which client or agent runs inside: brokered credentials, audit trail, revocation, trusted image, policy approval.
Exit: every zero action and sandbox egress decision is in the audit log; `zero revoke` stops push within seconds; tokens come from OpenBao and rotate without breaking running processes.

| ID | Feature | Arch | Status |
|---|---|---|---|
| F23 | Per-sandbox audit log (OpenShell events + zero actions) | §18, item 11 | todo |
| F24 | `zero revoke` soft/hard/quarantine, leftover-token cleanup | §19, item 12, P21 | todo |
| F25 | OpenBao as GitLab credential broker | §8, item 5, C44 | todo |
| F19 | Signed dev image on ghcr, pulled by digest | ADR 0021, §9 | todo |
| F20 | Policy widening prompt (`.zero/policy.yaml`) | §7, 07a | todo |

Order: F23 → F24 → F25 → F19 → F20.
