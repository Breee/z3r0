## 10. Deployment topology

Z3R0 has three logical zones, with OpenShell providing the execution plane.

```text
                         CENTRAL / SHARED

              ┌─────────────────────────────────┐
              │ Z3R0 Control Plane              │
              │                                 │
              │ Sessions / approvals / risk     │
              │ Semantic capability policy      │
              │ Organization Context            │
              │ LiteLLM / AI Gateway            │
              │ Credential backend (OpenBao)    │
              │ SPIRE Server / trust root       │
              │ Audit / OTel / Langfuse / SIEM  │
              └───────────────┬─────────────────┘
                              │
                  identity / policy / audit
                              │
           ┌──────────────────┼──────────────────┐
           │                                     │
           ▼                                     ▼

   DEVELOPER / LOCAL GATEWAY             KUBERNETES / REMOTE GATEWAY

┌──────────────────────────┐          ┌──────────────────────────────┐
│ IDE / OpenCode           │          │ OpenShell Gateway           │
│ Z3R0 plugin/extension    │          │ Kubernetes compute driver   │
│ OpenShell local Gateway  │          │ Z3R0 integration/adapters   │
│ Podman or MicroVM        │          │ SPIRE integration           │
│ Z3R0 middleware          │          │ OTel / OCSF export          │
└──────────────────────────┘          └──────────────┬───────────────┘
                                                    │
                                      Kubernetes-native controls
                                      RBAC / Admission / Cilium /
                                      Kyverno / Tetragon
```

### 10.1 Developer laptop

The developer workstation runs the UX integration plus a local OpenShell gateway/runtime. Normal enterprise target:

- OpenCode plugin and/or VS Code extension for session UX;
- OpenShell Gateway;
- Podman compute driver for normal rootless sandboxing;
- optional MicroVM driver for stronger isolation;
- Z3R0 OpenShell middleware implementing Phantom Proxy sanitization;
- local signed policy/session cache as needed;
- optional local telemetry forwarder.

The IDE/plugin is not the security boundary. The OpenShell sandbox/supervisor plus downstream native controls are.

### 10.2 Kubernetes / shared remote execution

For remote or fleet execution, deploy OpenShell Gateway with the Kubernetes compute driver. Each agent workload starts without ambient application/cluster/cloud credentials. Existing Kubernetes controls remain authoritative:

```text
Kubernetes RBAC
ValidatingAdmissionPolicy / Kyverno
Cilium / NetworkPolicy
Tetragon
```

Z3R0 should avoid inventing a mandatory extra cluster proxy if OpenShell plus native Kubernetes controls already provide the required enforcement. Add a Z3R0 cluster-side component only where a concrete semantic/sanitization requirement cannot be satisfied by those layers.

### 10.3 Central/shared services

Central services provide:

- Z3R0 logical-session/accountability service;
- semantic capability policy and approvals;
- Organization Context / service map integration;
- LiteLLM;
- OpenBao or another credential backend;
- SPIRE Server / trust infrastructure;
- audit correlation, OpenTelemetry, Langfuse and SIEM;
- policy/signing infrastructure.

### 10.4 Control-plane availability

Local development should continue for already-authorized local actions when central services are temporarily unavailable. New remote authority, new permission expansion, security-epoch creation/renewal beyond permitted bounds, or one-shot approvals must fail closed according to policy.

### 10.5 Production principle

There is no normal `prod writer agent` in Z3R0 v1.

```text
DEV / QA
→ agent write according to normal policy

PROD
→ no mutation
→ explicitly approved one-shot diagnostic reads only
→ sanitize before result reaches developer/agent endpoint
```

Humans and controlled GitOps/CI delivery systems change production.

---

