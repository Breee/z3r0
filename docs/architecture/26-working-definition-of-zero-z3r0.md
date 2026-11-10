## 26. Working definition of zero / Z3R0

> **zero / Z3R0 is an opinionated zero-trust DevOps/GitOps control layer for AI coding and operations agents, built on NVIDIA OpenShell as its primary secure execution substrate.**

Z3R0 treats agents like developers/operators, assumes they can be corrupted by untrusted input or supply-chain compromise, and composes OpenShell runtime containment with existing GitLab, Kubernetes, GitOps and cloud-native controls.

OpenShell supplies the hardened sandbox, network/provider mediation and runtime policy foundation. Z3R0 adds the enterprise workflow semantics that are intentionally outside a generic sandbox runtime:

- mandatory human accountability;
- logical sessions and sender-constrained security epochs;
- workspace = set of repositories;
- semantic Git/GitLab/Kubernetes/container capabilities;
- human approval and one-shot out-of-scope grants;
- Organization Context and risk escalation;
- production diagnostics without production mutation;
- Phantom Proxy sanitization semantics;
- signed agent commit provenance;
- cross-system audit/revocation;
- opinionated DevOps/GitOps delivery rules.

The simplest expression is:

```text
Agent gets a task.
Z3R0 gives it only the capabilities required for that task.
OpenShell contains the code that performs the task.
Native DevOps/GitOps/cloud-native systems enforce the real platform boundaries.
The agent never gains ambient authority merely because it can execute code.
```

### Implementation principle

```text
Do not rebuild OpenShell.
Do not rebuild GitLab.
Do not rebuild Kubernetes security.
Do not rebuild GitOps.

Build the opinionated agent-control layer that composes them safely.
```

---

