## 1. Design premise

An AI agent is normally **not malicious**. It is a trusted productivity tool operating on behalf of a developer or operator.

However, the architecture must assume that the agent can become **temporarily corrupted at runtime** by untrusted instructions or content, including:

- prompt injection in issues, documentation, source code, logs, websites, tickets, or chat messages;
- malicious or compromised MCP/tool responses;
- poisoned dependencies or build scripts;
- malicious container images or Dockerfiles;
- compromised skills/plugins;
- model/provider compromise;
- manipulated CI artifacts or API responses.

The primary security question is therefore:

> **What can a fully corrupted agent achieve with the capabilities currently assigned to it?**

### Foundational engineering principle

Z3R0 does **not** invent a separate engineering universe for AI agents. Agent coders and operator agents are treated like developers and operators and must use the same established DevOps, GitOps, cloud-native, least-privilege, and zero-trust controls that already protect human workflows.

> **Treat agents as developers, not as a new infrastructure class.**

Use existing controls first:

- GitLab protected branches, merge requests, CODEOWNERS, and CI;
- GitOps for declarative environment changes;
- Kubernetes RBAC and admission;
- native Kubernetes policy and/or Kyverno where appropriate;
- Cilium for network policy and egress enforcement;
- Tetragon for runtime visibility/enforcement;
- SPIFFE/SPIRE for workload identity;
- OpenBao for short-lived credentials;
- rootless containers / microVMs for execution isolation;
- OpenTelemetry, SIEM, and Langfuse for evidence.

Z3R0 adds controls only where agent-specific risks create gaps: runtime corruption, credential mediation, constrained agent sessions, egress restriction, capability scoping, trace correlation, and revocation.

The answer must be bounded by external enforcement, not by hoping that the model recognizes the attack.

The target failure mode is:

```text
malicious input
    ↓
agent follows malicious instruction
    ↓
runtime / capability boundary
    ↓
DENIED + AUDITED
```

not:

```text
malicious input
    ↓
agent detects the attack itself
    ↓
ignores it
```

Model-level prompt-injection defenses and guardrails are useful defense-in-depth, but they are **not** the primary security boundary.

---

