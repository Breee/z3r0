## 9. Runtime isolation — OpenShell foundation

### 9.1 Trust model

OpenShell is the runtime boundary where agent-controlled code executes. Z3R0 assumes everything inside the sandbox may be compromised.

```text
agent + repository code + build/test scripts
                │
                ▼
        OpenShell sandbox
                │
       protected supervisor channel
                │
                ▼
       OpenShell supervisor
                │
        policy / credentials / proxy
```

The supervisor is outside the agent workload. Z3R0 must not place its authoritative approval, credential or sanitization logic inside the agent-controlled process tree.

### 9.2 Filesystem and process isolation

Z3R0 relies on OpenShell's kernel/runtime controls instead of implementing a separate `OpenShell sandbox` layer. On supported Linux runtimes, OpenShell provides overlapping controls including:

- Landlock-based filesystem restrictions;
- immutable non-root workload identity;
- zero Linux capabilities;
- `no_new_privs`;
- seccomp-based syscall restrictions;
- supervisor-mediated network/DNS operations.

The allowed workspace is the only normal read/write development area. Host credentials and unrelated repositories remain outside the sandbox.

Default mode permits arbitrary local processes **inside the sandbox** because real projects need shells, interpreters, tests and build scripts:

```text
bash
make
go test ./...
python
node
custom project scripts
```

An optional **strict mode** may additionally restrict executable/command patterns for highly standardized environments, but command allowlisting is defense-in-depth rather than the primary containment boundary.

### 9.3 Workspace isolation

Preferred mode:

```text
developer checkout
      ↓
isolated worktree/copy/volume
      ↓
OpenShell sandbox
```

Direct host bind mounts may be offered for convenience only when their weaker isolation is explicitly accepted. The hardened/default enterprise target should keep agent writes in an isolated workspace that the IDE can attach to.

### 9.4 Network enforcement

The agent sandbox must not have an unrestricted network path. OpenShell's supervisor/network policy is the primary runtime egress boundary.

```text
Agent sandbox
      ↓
OpenShell network mediation
      ↓
policy decision
      ├─ authenticated service → Phantom Proxy/provider path
      ├─ trusted public dependency mirror → allow
      ├─ policy-approved public fetch → allow / optional scanner
      └─ unknown destination → deny/request
```

For MicroVM sandboxes, prefer OpenShell's VM networking model where workload traffic is forced through the supervisor rather than relying on a normal guest NIC.

### 9.5 Container build/run

The agent never receives the developer's normal Docker socket. [Z-BUILD-1]

```text
FORBIDDEN DEFAULT:
/var/run/docker.sock
```

Container build/run occurs inside or through the selected OpenShell sandbox/runtime with:

- rootless execution where available; [Z-BUILD-2]
- isolated container/image storage; [Z-BUILD-3]
- restricted build context; [Z-BUILD-4]
- no arbitrary host mounts; [Z-BUILD-5]
- no host network; [Z-BUILD-6]
- no privileged containers; [Z-BUILD-7]
- no device passthrough unless explicitly justified; [Z-BUILD-8]
- resource limits; [Z-BUILD-9]
- controlled dependency/registry access; [Z-BUILD-10]
- no raw build secrets. [Z-BUILD-11]

For more hostile repositories or Dockerfiles, select the OpenShell MicroVM execution profile. [Z-BUILD-12]

### 9.6 Integration testing

Prefer an isolated test network:

```text
isolated test network

 test-runner
      │
      ▼
 payments-api:8080
      │
      ▼
 postgres
```

Do not expose agent-started test services on the developer host unless explicitly required.

### 9.7 Dependency access

Dependency access is a distinct capability class. Enterprise profiles should prefer approved internal infrastructure:

```text
Go       → GOPROXY / internal Artifactory/Nexus
npm      → internal registry
pip      → internal index
OCI      → internal registry/mirror
apt/yum  → internal OS package mirror
```

The organization declares approved mirrors; Z3R0/OpenShell inject the tool-specific runtime configuration. Direct public registry access is an explicit risk acceptance, not the enterprise default.

---

