## 13. Reference scenario B — Go developer with GitLab + containers

### Goal

> I develop a Go application in GitLab. The agent should edit code, run tests, build and test containers, push the feature branch, and inspect GitLab CI safely.

Assume:

```text
Repository:
gitlab.com/acme/payments-api

Branch:
feature/add-refund-endpoint
```

### 13.1 Default capabilities

```yaml
workspace:
  repository: acme/payments-api

filesystem:
  read_write:
    - workspace/**

network:
  allow:
    - gitlab.com
    - proxy.golang.org
    - sum.golang.org
    - registry.example.com

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

approval:
  gitlab.mr.merge: human
```

Explicitly denied by default:

```text
git.push main/master
git.force_push
git.tag.create
gitlab.pipeline.cancel
gitlab.release.create
container.registry.push
arbitrary host mounts
privileged containers
```

### 14.2 Development loop

```text
Developer
  │
  │ "Implement issue #142"
  ▼
Agent
  ├─ inspect repository
  ├─ inspect GitLab issue
  ├─ edit Go code
  ├─ gofmt
  └─ go test ./...
```

All local execution remains inside the runtime boundary.

### 14.3 GitLab issue / MR / pipeline access

The agent may use MCP or `glab`.

Examples:

```bash
glab issue view 142
glab mr view
glab pipeline status
glab pipeline view
glab job trace <job>
```

The capability is not `glab` itself.

```text
gitlab.pipeline.read → ALLOW
gitlab.pipeline.retry → DENY or approval
gitlab.pipeline.cancel → DENY
gitlab.release.create → DENY
```

Authentication is supplied by the credential boundary using short-lived authority. The agent does not receive a persistent GitLab token.

### 14.4 Local container build

Agent requests:

```bash
docker build -t payments-api:test .
```

The architecture interprets this as:

```text
container.build
context = workspace/payments-api
Dockerfile = workspace/payments-api/Dockerfile
```

Flow:

```text
Agent
  ↓
OpenShell sandbox
  ↓
container.build policy
  ├─ context allowed?
  ├─ Dockerfile allowed?
  ├─ registry allowed?
  ├─ network allowed?
  └─ build secrets allowed?
       ↓
OpenShell Podman/build runtime
```

The developer's host Docker socket is never mounted into the agent environment.

### 14.5 Build network and base images

Example policy:

```yaml
container:
  build:
    context:
      - workspace/**

    registries:
      pull:
        - registry.example.com
        - docker.io/library

    network:
      allow:
        - proxy.golang.org
        - sum.golang.org
        - registry.example.com

    host_mounts: false
    privileged: false
```

A Dockerfile is treated as executable code, not benign configuration.

### 14.6 Private modules/build secrets

Bad:

```text
GITLAB_TOKEN=<real token>
```

or:

```dockerfile
ARG GITLAB_TOKEN
```

Preferred flow:

```text
Build step
  ↓
requests private-module-read capability
  ↓
Phantom Proxy
  ↓
OpenBao
  ↓
short-lived credential / secret mount
  ↓
approved GitLab destination only
```

### 14.7 Container testing

After build:

```text
payments-api:test
```

Agent requests `container.run`.

Safe defaults:

```yaml
container:
  run:
    privileged: false
    host_network: false
    host_pid: false
    devices: false
    host_mounts: false
    docker_socket: false

    capabilities:
      drop:
        - ALL

    network:
      isolated: true

    resources:
      cpu: 2
      memory: 1Gi
      pids: 256
```

Integration dependencies run in the same isolated test network.

### 14.8 Git push

```bash
git push origin feature/add-refund-endpoint
```

Flow:

```text
Agent
  ↓
OpenShell sandbox
  ↓
Phantom Proxy
  ├─ repo allowed?
  ├─ push allowed?
  └─ branch = feature/* ?
       ↓
OpenBao
  ↓
short-lived GitLab authority
  ↓
GitLab
```

Attempt:

```bash
git push origin HEAD:main
```

Result:

```text
DENY
```

before privileged authentication is made available.

### 14.9 GitLab CI feedback loop

```text
write code
  ↓
go test
  ↓
build container
  ↓
run local integration tests
  ↓
commit
  ↓
push feature branch
  ↓
GitLab CI
  ↓
glab pipeline status / view
  ↓
failed? ── yes → inspect logs → fix → repeat
  │
  no
  ↓
create MR
```

MR creation can be automatic. Merge to `main` can remain protected by normal GitLab approval rules and optionally explicit agent approval policy.

---

