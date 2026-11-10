# Sprint 06 — evidence & control (Phase 2)

Goal: every sandbox action is recorded and can be stopped; credentials come from a broker.
Exit: audit log covers the sprint 05 loop; `zero revoke` stops push within seconds; token rotation does not break running agents.

| ID | Feature | Arch | Status |
|---|---|---|---|
| F23 | Per-sandbox audit log (OpenShell events + zero actions) | §18, item 11 | todo |
| F24 | `zero revoke` soft/hard/quarantine, leftover-token cleanup | §19, item 12, P21 | todo |
| F25 | OpenBao as GitLab credential broker | §8, item 5, C44 | todo |

Order: F23 → F24 → F25.
