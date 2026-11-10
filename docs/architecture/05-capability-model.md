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

```text
k8s.resource.read
k8s.resource.create
k8s.resource.patch
k8s.resource.delete

k8s.pod.logs
k8s.pod.exec

k8s.workload.restart

k8s.secret.read
k8s.rbac.modify
k8s.crd.modify
```

Resource scope:

```text
cluster
namespace
apiGroup
kind
name
```

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

