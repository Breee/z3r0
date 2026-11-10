# zero architecture

Source of truth for the architecture (draft v0.9, split from the former `Z3R0_ARCHITECTURE_v0.9_OPEN_SHELL.md`; section numbers `§N` are unchanged).
Requirements carry inline markers like `[Z-P0-8]` and are listed in [requirements.yaml](requirements.yaml). Decisions: [../adr/](../adr/README.md). Progress: GitHub issues, labelled `req:<ID>`.

## arc42 mapping

| arc42 chapter | Files |
|---|---|
| 1 Introduction and goals | 00, 01, 26 |
| 2 Constraints | 02 (invariants Z1–Z6), 23 (derived rules), A (OpenShell assumptions) |
| 3 Context and scope | 10, 11 |
| 4 Solution strategy | 03, 21 |
| 5 Building blocks | 04, 05, 08, 15 |
| 6 Runtime view | 12, 13, 22 |
| 7 Deployment view | 06, 09, 10 |
| 8 Crosscutting concepts | 07, 16, 17, 18, 19, 20 |
| 9 Decisions | [../adr/](../adr/README.md) |
| 10 Quality / risk | 14 |
| 11 Open risks / questions | 24 |
| Rollout | 25 |

## Context (C4)

```mermaid
C4Container
  Person(dev, "Developer")
  System_Boundary(host, "Developer host") {
    Container(cli, "zero CLI / IDE", "UX only, not a boundary")
    Container(gw, "OpenShell gateway", "compute driver: Podman / MicroVM / K8s")
    Container_Boundary(sb, "OpenShell sandbox") {
      Container(sup, "Supervisor", "policy, Landlock, network proxy, placeholder substitution")
      Container(agent, "Agent", "untrusted process tree")
    }
  }
  Container(phantom, "Phantom Proxy", "authenticated remote access, credential binding")
  System_Ext(bao, "OpenBao", "real secrets")
  System_Ext(llm, "AI control plane", "LLM gateway")
  System_Ext(forge, "GitLab", "branches, MRs, CI")
  Rel(dev, cli, "uses")
  Rel(cli, gw, "creates sandboxes")
  Rel(gw, sb, "runs")
  Rel(agent, sup, "all egress via")
  Rel(sup, phantom, "mediated requests")
  Rel(phantom, bao, "fetches credentials")
  Rel(sup, llm, "inference")
  Rel(phantom, forge, "narrow authority")
```

## Files

- [zero / Z3R0 — Zero-Trust Agent Stack](00-introduction.md)
- [1. Design premise](01-design-premise.md)
- [2. Core security invariants](02-core-security-invariants.md)
- [3. Architectural primitives](03-architectural-primitives.md)
- [4. Core components](04-core-components.md)
- [5. Capability model](05-capability-model.md)
- [6. Risk classes and execution isolation](06-risk-classes-and-execution-isolation.md)
- [7. Policy: one source, many enforcement points](07-policy-one-source-many-enforcement-points.md)
- [7a. Policy sources and lifecycle](07a-policy-sources-and-lifecycle.md)
- [8. Credential model](08-credential-model.md)
- [9. Runtime isolation — OpenShell foundation](09-runtime-isolation-openshell-foundation.md)
- [10. Deployment topology](10-deployment-topology.md)
- [11. Developer laptop integration](11-developer-laptop-integration.md)
- [12. Reference scenario A — Kubernetes operator](12-reference-scenario-a-kubernetes-operator.md)
- [13. Reference scenario B — Go developer with GitLab + containers](13-reference-scenario-b-go-developer-with-gitlab-containers.md)
- [14. Threat model and attack paths](14-threat-model-and-attack-paths.md)
- [15. AI control plane](15-ai-control-plane.md)
- [16. Kubernetes authorization model](16-kubernetes-authorization-model.md)
- [17. GitLab authorization model](17-gitlab-authorization-model.md)
- [18. Observability and evidence](18-observability-and-evidence.md)
- [19. Revocation and incident response](19-revocation-and-incident-response.md)
- [20. DevOps controls vs runtime controls](20-devops-controls-vs-runtime-controls.md)
- [21. Canonical architecture](21-canonical-architecture.md)
- [22. Session & identity protocol — resolved snapshot](22-session-identity-protocol-resolved-snapshot.md)
- [23. Derived design rules](23-derived-design-rules.md)
- [24. Open questions / validation work](24-open-questions-validation-work.md)
- [25. Initial rollout](25-initial-rollout.md)
- [26. Working definition of zero / Z3R0](26-working-definition-of-zero-z3r0.md)
- [Appendix A — OpenShell assumptions used by this draft](A-openshell-assumptions.md)
- [Scenario: Go + containers + GitLab](../scenarios/go-container-gitlab.md)
