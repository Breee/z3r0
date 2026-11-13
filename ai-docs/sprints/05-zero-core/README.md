# Sprint 05 — control plane PoCs (Phase 2)

Goal: prove the building blocks of arch §27 (ADR 0025) as rootless podman containers (Quadlet, dedicated user), same images later in k8s.
Rule: each PoC is a gate. Fail → caveat + upstream report, stop, ask (AGENTS.md).

| ID | PoC | Pass condition | Status |
|---|---|---|---|
| P1 | Gateway as container | `zero up` unchanged; gateway sees only its own podman socket | todo |
| P2 | SPIRE server + agent in containers, attest sandbox supervisor | supervisor gets JWT-SVID for exactly its sandbox; agent reaches neither socket nor SVID | done 2026-11-13, `pocs/08-spire-podman` (#83) |
| P3 | Minimal zero-cp token endpoint | supervisor injects test token; agent never sees it; unknown SVID denied; decision log has both | todo |
| P4 | OpenBao, zero-cp authenticates with own SVID | read works with SVID, denied without | todo |
| P5 | GitLab end to end | push via grant; revoke → push fails ≤60s, token revoked at GitLab; restart cleans leftover tokens | todo |
| P6 | Same images on kind | P2–P5 pass; k8s differences documented | todo |

Order: P1 → P6. Former F19–F25 drafts superseded by this plan.
