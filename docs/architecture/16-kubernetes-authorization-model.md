## 16. Kubernetes authorization model

The agent-side policy is not the final authority.

Cluster access should require all applicable checks:

```text
agent capability policy
        AND
short-lived cluster identity
        AND
Kubernetes RBAC
        AND
admission/policy
```

This supports defense-in-depth against:

- local policy bugs;
- compromised agents;
- compromised developer workstations;
- accidental over-granting.

Z3R0 should reuse existing cloud-native enforcement rather than duplicate it:

| Question | Primary enforcement |
|---|---|
| May this agent request a Kubernetes operation? | Z3R0 capability policy |
| May this identity call this Kubernetes API? | Kubernetes RBAC |
| Is the requested object/configuration safe? | ValidatingAdmissionPolicy and/or Kyverno |
| May this workload reach this network destination? | Cilium / Kubernetes network policy |
| Is the workload/process exhibiting forbidden runtime behavior? | Tetragon / runtime security |
| May a credential be issued/used for this target? | Z3R0 Phantom Proxy + OpenBao |

Native Kubernetes policy should be preferred for straightforward admission rules. Kyverno is optional where richer policy, image verification, attestation, reporting, or policy-management capabilities are useful.

Production preference:

```text
read live state directly
change desired state via GitOps
normal agent identities have no direct production mutation capability
```

---

