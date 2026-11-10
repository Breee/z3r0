## 6. Risk classes and execution isolation

Risk and execution location are separate dimensions.

### 6.1 Risk classes

| Class | Meaning | Examples | Default response |
|---|---|---|---|
| **C0 — Read** | No mutation | read repo, logs, issues, pipelines, cluster state | automatic |
| **C1 — Local mutation** | Change only sandbox/workspace | edit code, commit, build image locally | automatic inside sandbox |
| **C2 — Reversible remote mutation** | Remote change with limited blast radius | push feature branch, create MR, mutate assigned DEV/QA workloads | policy + short-lived authority |
| **C3 — Protected workflow transition** | Action that crosses a protected delivery boundary | request promotion, request protected-branch merge, request production change | agent may prepare/request; human or controlled delivery system performs/approves |
| **C4 — Production/security mutation** | Direct production mutation or change to security boundaries | prod write/delete/exec, RBAC, secrets, CRDs, DB drop, storage deletion | unavailable to normal agents; separate break-glass/system identity only |

### 6.2 Execution isolation profiles

Risk class and sandbox strength remain separate dimensions, but the implementation is now based on OpenShell compute drivers rather than separate Z3R0 sandbox products.

| Level | OpenShell runtime | Typical use |
|---|---|---|
| **E1 — Rootless container sandbox** | Podman driver | normal interactive developer work on trusted first-party repositories |
| **E2 — Hardened container/build sandbox** | Podman/OpenShell with stricter filesystem/network/resource policy and isolated build runtime | local container build/run and integration testing |
| **E3 — VM sandbox** | OpenShell MicroVM driver | untrusted repositories, hostile build scripts, third-party code, stronger endpoint isolation |
| **E4 — Cluster sandbox** | OpenShell Kubernetes driver | remote/shared/fleet agents and centrally managed execution |

Examples:

```text
risk=C1, execution=E2
→ local container build

risk=C2, execution=E1/E4
→ mutate assigned QA workload according to policy

risk=C1, execution=E3
→ build unknown third-party repository
```

OpenShell provides the common sandbox API across these execution profiles. Z3R0 selects the profile according to policy/risk rather than maintaining separate sandbox implementations.

---

