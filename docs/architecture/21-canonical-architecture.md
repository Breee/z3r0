## 21. Canonical architecture

OpenCode/VS Code provide UX and context. OpenShell provides the agent execution/security substrate. Z3R0 provides the opinionated session/capability/approval layer.

```text
                              HUMAN
                                │
                         Enterprise IdP
                                │
                                ▼
                     Z3R0 Logical Session
                     + Security Epoch
                                │
                                ▼
                   Semantic Capability Policy
                                │
          ┌─────────────────────┼─────────────────────┐
          │                     │                     │
          ▼                     ▼                     ▼
      LiteLLM             Z3R0 Approval/Risk     Organization Context
   LLM / MCP / A2A           / one-shot grants    / service map
          │                     │                     │
          └──────────────┬──────┴─────────────────────┘
                         ▼
                   OpenShell Gateway
                         │
             ┌───────────┼─────────────┐
             │           │             │
             ▼           ▼             ▼
         Policy      Providers      Middleware
      fs/proc/net    credentials    Z3R0 Phantom
             │           │          sanitization
             └───────────┼─────────────┘
                         ▼
                  OpenShell Sandbox
                         │
               ┌─────────┼──────────┐
               ▼         ▼          ▼
              git      kubectl   container/build
               │         │          │
               └─────────┼──────────┘
                         ▼
               Native target controls
               ┌─────────┴─────────┐
               ▼                   ▼
            GitLab             Kubernetes
      protected branches       RBAC/admission
      CI/MR/GitOps             Cilium/Kyverno/
                               Tetragon

     Everything → OCSF / OpenTelemetry / Z3R0 Audit / Langfuse
```

### Stronger-risk execution

```text
same Z3R0 session/capability model
            ↓
OpenShell MicroVM driver
            ↓
agent + untrusted code
            ↓
all network forced through supervisor
```

### Remote/fleet execution

```text
same Z3R0 session/capability model
            ↓
OpenShell Kubernetes driver
            ↓
Kubernetes sandbox workload
            ↓
native cluster security controls
```

---

