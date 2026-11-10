---
status: proposed
date: 2026-11-10
deciders: bree
requirements: [Z-INV-3, Z-INV-4, Z-INV-6]
---
# 0025 — One zero control plane, run locally (basic) or centrally (enterprise)

## Context
§22 requires runtime identity (SPIFFE), session + security epoch, central revocation and evidence. Today the CLI mints a static per-sandbox GitLab token (ADR 0020); rotation misses running processes (C44); revocation depends on the laptop. OpenShell 0.1.4 supports SPIFFE JWT-SVIDs for supervisors and per-request OAuth token grants (`token_grant`).

## Decision
Build one control plane (`zero-cp`): session/epoch store, capability decision, OAuth 2.0 token endpoint for OpenShell token grants, revocation, decision log. SPIRE issues supervisor identities, OpenBao holds forge root credentials. Basic mode runs it on the laptop under a dedicated user; enterprise mode runs the same software centrally. Design: `docs/architecture/27-control-plane.md`.

## Consequences
- No static credentials in gateway or sandbox; every credentialed request is authorized per epoch and logged.
- Revocation effect ≤ grant cache TTL (60s).
- More local services (SPIRE, OpenBao, zero-cp) for `zero install`/`zero doctor`.
- Basic mode trusts the laptop's root; enterprise mode does not.
- Supersedes ADR 0020 once implemented.
