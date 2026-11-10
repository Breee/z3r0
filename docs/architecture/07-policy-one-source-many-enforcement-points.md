## 7. Policy: one source, many enforcement points

Z3R0 policy is semantic and opinionated. OpenShell policy is the main runtime representation, while target systems retain their own native policy.

Example high-level Z3R0 policy:

```yaml
agent: go-developer

workspace:
  repositories:
    - gitlab.com/acme/payments-api

capabilities:
  - git.read
  - git.commit
  - git.push
  - gitlab.issue.read
  - gitlab.mr.read
  - gitlab.mr.create
  - gitlab.pipeline.read
  - container.image.pull
  - container.build
  - container.run

constraints:
  git.push:
    branches:
      - feature/*
      - fix/*
    deny_protected: true

  production:
    mutate: false
    diagnostics: one_shot_approved
```

The Z3R0 adapter/compiler renders that intent into existing enforcement systems:

| Enforcement point | Native control |
|---|---|
| **OpenShell filesystem/process** | Landlock/process/seccomp sandbox policy |
| **OpenShell network policy** | destination, port, binary and optional L7 rules |
| **OpenShell providers** | credential placeholder/substitution, endpoint binding |
| **OpenShell middleware** | Z3R0 response sanitization / Phantom Proxy behavior |
| **LiteLLM** | model/MCP visibility, permissions, budgets and guardrails |
| **Credential authority** | credential role, resource scope and TTL |
| **GitLab** | protected branches, repository permissions, MR approvals, CI policy |
| **Kubernetes** | RBAC, admission and namespace/resource scoping |
| **Cilium/Kyverno/Tetragon** | native network/admission/runtime enforcement |

### Native controls first

Z3R0 does not attempt to reproduce GitLab branch protection, Kubernetes RBAC, admission control, network policy or runtime security. It expects organizations to use mature DevOps/GitOps/cloud-native controls and adds the agent-specific session, capability, mediation and audit layers around them.

### OpenShell Policy Advisor / Prover

Where OpenShell's Policy Advisor or Policy Prover applies, Z3R0 should reuse it rather than build competing low-level policy machinery.

A future permission expansion may flow like:

```text
agent requests capability
      ↓
Z3R0 semantic policy/risk evaluation
      ↓
translate to OpenShell policy delta
      ↓
OpenShell policy proof / maximum-boundary check
      ↓
human approval when required
      ↓
scoped or one-shot grant
```

OpenShell's current advisor/prover surface does not replace Z3R0's broader semantic approval model; it is a lower-level safety mechanism underneath it.

### Fail closed

No matching capability means deny. Failure of policy, credential issuance, proof or approval must not silently expand authority.

---

