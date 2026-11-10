## 4. Core components

### 4.1 Architectural ownership

Z3R0 is intentionally **not a sandbox product**. OpenShell provides the low-level secure agent runtime. Z3R0 provides the opinionated DevOps/GitOps control model above it.

```text
Z3R0
├─ human-accountable logical sessions
├─ security epochs
├─ semantic capabilities
├─ approval + one-shot grants
├─ Organization Context + risk enrichment
├─ Git / GitLab / Kubernetes / container opinions
├─ production diagnostic policy
├─ Git commit provenance
└─ cross-system audit correlation

OpenShell
├─ sandbox lifecycle
├─ filesystem isolation
├─ process/syscall isolation
├─ network enforcement
├─ provider credentials / placeholder substitution
├─ supervisor proxy
├─ middleware
├─ policy hot-reload where supported
├─ compute drivers
│  ├─ Podman
│  ├─ MicroVM
│  └─ Kubernetes
└─ runtime security events
```

### 4.2 Components

| Component | Responsibility | Status |
|---|---|---|
| **OpenShell** | Primary sandbox/runtime substrate: process/filesystem/network containment, provider credential substitution, compute drivers, middleware and runtime audit | Required |
| **SPIFFE / SPIRE** | Short-lived workload/runtime identity; basis for Z3R0 security-epoch identity and commit provenance | Required |
| **Z3R0 Control Plane** | Logical sessions, human accountability, security epochs, semantic capabilities, approvals, one-shot grants, revocation and risk context | Required |
| **Z3R0 capability policy** | Opinionated high-level policy compiled into OpenShell and native target controls | Required |
| **Phantom Proxy** | Z3R0 trust-boundary concept implemented primarily through OpenShell provider credential substitution + supervisor proxy + Z3R0 middleware; hides credentials and sensitive response data | Required |
| **Credential authority/backend** | Mints or owns real upstream credentials; **OpenBao is the preferred initial enterprise backend**, but is not the sandbox-facing API | Required capability; backend may vary |
| **LiteLLM** | Central LLM/MCP/A2A control plane, budgets, tool visibility, guardrails and tool routing | Required |
| **Kubernetes RBAC + admission** | Final cluster-side authorization and policy enforcement | Required for Kubernetes scenario |
| **ValidatingAdmissionPolicy / Kyverno** | Enforce safe workload/resource configuration and supply-chain policy | Native policy preferred; Kyverno optional |
| **Cilium** | Default-deny network policy and egress controls in Kubernetes | Recommended |
| **Tetragon** | Runtime/kernel visibility and optional enforcement in Kubernetes | Recommended defense-in-depth |
| **Langfuse + OpenTelemetry + security audit** | End-to-end evidence, cost, traces, denials and incident reconstruction | Required |
| **Organization Context** | Optional service map, ownership, criticality, dependencies and data-classification context used to enrich/escalate risk | Optional but valuable |

### 4.3 Phantom Proxy implementation model

**Phantom Proxy remains a Z3R0 architectural concept, not necessarily a separate standalone proxy binary.**

For the initial implementation, its responsibilities should be composed from OpenShell primitives:

```text
OpenShell provider/profile
→ opaque/placeholder credential inside sandbox
→ endpoint-bound real credential substitution outside sandbox

OpenShell supervisor proxy + Z3R0 middleware
→ request mediation
→ response sanitization
→ secret/connection-string redaction
→ audit
```

Z3R0-specific middleware supplies the high-assurance functionality OpenShell does not provide out of the box, for example:

- Kubernetes-spec safe projections;
- production-log sanitization;
- connection-string parsing and partial redaction;
- organization-specific secret detectors;
- cumulative disclosure tracking;
- optional private-model secret/injection detection.

### 4.4 Important component boundaries

- **OpenShell** controls *where and how agent-controlled code executes* and mediates its network/provider access.
- **Z3R0** controls *which semantic capabilities a human-accountable session is intended to have*.
- **LiteLLM** controls *which AI/model/MCP operations are visible and callable*.
- **OpenBao or another credential authority** owns/mints real credentials; credentials are brokered into OpenShell providers and never delivered to the agent/developer.
- **GitLab** independently enforces protected branches, repository permissions, CI and merge workflows.
- **Kubernetes** independently enforces RBAC/admission.
- **Cilium/Kyverno/Tetragon** remain native cloud-native controls rather than Z3R0 replacements.

No single component is trusted to provide all security controls.

---

