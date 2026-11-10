## 3. Architectural primitives

The stack is built around four primitives:

```text
Identity → Capability → Boundary → Evidence
```

### Identity

**Who is acting?**

Primary mechanism: **SPIFFE / SPIRE** for workload/runtime identity, combined with the Z3R0 logical-session and security-epoch model defined later in this document. Human identity is normally federated from the enterprise IdP.

### Capability

**What may this identity do to which resource?**

Z3R0 capabilities are semantic and opinionated, for example:

```text
git.push
gitlab.pipeline.read
k8s.logs.read
container.build
```

Z3R0 translates those capabilities into the native enforcement mechanisms of OpenShell and the target systems instead of replacing those systems.

### Boundary

**Where is authority contained?**

**OpenShell is the primary execution and runtime-security substrate for Z3R0.** It provides the agent sandbox, filesystem/process isolation, network mediation, provider credential substitution, compute-runtime abstraction, and supervisor middleware hooks.

Depending on the required isolation level, OpenShell may use:

- rootless **Podman** for normal developer workstations;
- the **MicroVM** driver for stronger isolation;
- the **Kubernetes** driver for remote/shared agent execution;
- Docker where appropriate for local development or evaluation.

Outside OpenShell, native security boundaries remain authoritative:

- GitLab protected branches, permissions, CI and merge requests;
- Kubernetes RBAC and admission;
- Cilium / NetworkPolicy;
- Kyverno or native CEL admission policy;
- Tetragon runtime policy;
- OpenBao or another credential authority;
- LiteLLM MCP/tool/model policy.

### Evidence

**What happened?**

Correlate:

- logical Z3R0 session and active security epoch;
- model calls;
- tool/MCP calls;
- process execution;
- OpenShell policy decisions and OCSF events;
- approvals and one-shot grants;
- credential substitutions/leases;
- network calls;
- GitLab operations;
- Kubernetes API calls;
- signed Git commit provenance.

Use OpenTelemetry plus security audit logs; Langfuse remains useful for model/agent traces, evaluation, cost, and prompt context.

---

