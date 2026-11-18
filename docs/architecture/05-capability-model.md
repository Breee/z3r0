## 5. Capability model

Capabilities are semantic operations with resource constraints.

### 5.1 Filesystem

```text
fs.read
fs.write
fs.create
fs.delete
```

Resources are paths relative to an approved workspace.

Example:

```yaml
filesystem:
  read_write:
    - /workspace/payments-api/**
  deny:
    - ~/.ssh/**
    - ~/.aws/**
    - ~/.kube/**
    - ~/.config/**
```

### 5.2 Process execution

```text
process.exec
```

For simple CLIs, arguments may be restricted.

For general shells/interpreters such as `bash`, `sh`, `python`, or `node`, assume they provide arbitrary code execution **inside the sandbox**. Do not attempt to infer perfect security from shell command parsing.

### 5.3 Network

```text
network.connect
network.listen
```

Resources:

```text
scheme + host + port
```

Default should be deny-by-default for sandboxed execution.

### 5.4 Git

```text
git.read
git.commit
git.push
git.force_push
git.tag.create
```

Resource scope:

```text
repository + branch/ref
```

### 5.5 GitLab

```text
gitlab.issue.read

gitlab.pipeline.read
gitlab.pipeline.run
gitlab.pipeline.retry
gitlab.pipeline.cancel

gitlab.mr.read
gitlab.mr.create
gitlab.mr.merge

gitlab.release.create
```

### 5.6 Kubernetes

Refined 2026-11-18 after the proof on a kind cluster (#123, [kubernetes-access.md](../scenarios/kubernetes-access.md) sections 9 and 10). Nothing of it is built in zero yet.

Rule: a capability is only offered when the cluster can enforce it exactly. Each capability is one RBAC Role `zero-k8s-<object>-<action>` that the cluster admin installs per namespace; zero binds the session's ServiceAccount to the Roles the policy lists. zero does not check requests itself.

| Capability | Allows | Enforced exactly | Default |
|---|---|---|---|
| `k8s.resource.read` | get/list/watch of workload objects (pods, events, services, deployments, jobs, ingresses, ...). No Secrets, no ConfigMaps | yes | offered |
| `k8s.pod.logs` | read pod logs | yes | offered |
| `k8s.config.read` | read ConfigMaps | yes | offered |
| `k8s.workload.scale` | change replicas of Deployments and StatefulSets | yes | offered |
| `k8s.pod.delete` | delete pods | yes | admin opt-in |
| `k8s.workload.restart` | rollout restart | **no**: RBAC `patch` allows any change; with an admission policy two fields still got through | not offered |

Changed against the first list:

| First list | Now | Reason |
|---|---|---|
| `k8s.resource.read` | kept, without Secrets and ConfigMaps; ConfigMaps are `k8s.config.read` | ConfigMaps often hold credentials |
| `k8s.pod.logs` | kept | |
| `k8s.workload.restart` | kept as a name, not offered | not exact (above) |
| `k8s.resource.create`, `.patch`, `.delete` | dropped | "any resource" is as wide as the human's own access. Changes go through git (GitOps). Narrow writes get their own capability (`k8s.workload.scale`, `k8s.pod.delete`) |
| `k8s.pod.exec` | never for an agent | code execution in the cluster with the pod's ServiceAccount and Secrets, outside the sandbox |
| `k8s.secret.read`, `k8s.rbac.modify`, `k8s.crd.modify` | never for an agent | no Role exists, so no policy can ask for them |
| new: `k8s.config.read`, `k8s.workload.scale`, `k8s.pod.delete` | | split out of read and patch |

Resource scope:

```text
cluster (kubeconfig context)
namespace
```

`apiGroup`, `kind` and `name` are no longer part of the policy. They are fixed by the Role behind the capability; the cluster admin can edit that Role or add own ones (`k8s.argo.read` ↔ Role `zero-k8s-argo-read`).

Example:

```yaml
kubernetes:
  - context: dev
    namespaces: [team-a, team-a-qa]
    capabilities: [k8s.resource.read, k8s.pod.logs]
  - context: prod
    namespaces: [team-a]
    capabilities: [k8s.resource.read]
```

Open: cluster-scoped reads (nodes, namespaces, CRDs) need a ClusterRole; whether an agent may hold a capability its human does not hold (PF2).

### 5.7 Containers

```text
container.image.pull
container.build
container.run
container.network
container.mount
container.device
container.registry.push
```

Do **not** model this as:

```text
docker = true
```

### 5.8 LLM / MCP / A2A

```text
model.invoke
mcp.tool.call
a2a.agent.call
```

Constrain by:

- model/provider;
- server/tool/agent;
- budget;
- data classification;
- argument policy;
- trust classification.

---

