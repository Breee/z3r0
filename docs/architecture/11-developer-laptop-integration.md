## 11. Developer laptop integration

### 11.1 Goal

Developers must be able to use Z3R0 without replacing their normal workflow. A secure agent session should still feel like:

```text
git clone
code .
agent
go test ./...
docker build .
glab pipeline status
git push
```

The security controls should be mostly invisible during routine development.

> **Secure by construction, convenient by default.**

### 11.2 Separate developer authority from agent authority

The developer's normal shell and the agent session are distinct principals even on the same laptop.

```text
Laptop
├── developer shell
│   └── normal human access according to company policy
│
└── Z3R0 agent session
    ├── constrained filesystem
    ├── constrained network
    ├── constrained credentials
    ├── constrained container runtime
    └── independent audit/session identity
```

A human may, for example, have interactive production access according to corporate policy while the agent remains production read-only. The human's ambient credentials must not automatically become available to the agent.

### 11.3 OpenCode integration

OpenCode is a natural integration point through a Z3R0 plugin plus a custom model/provider endpoint.

```text
OpenCode
   │
   ├─ Z3R0 plugin
   │   ├─ establish/refresh Z3R0 session
   │   ├─ discover repository + branch
   │   ├─ select effective developer profile
   │   ├─ expose session/capability status
   │   └─ propagate trace/session context
   │
   └─ custom LLM endpoint
       ↓
     Z3R0 AI Gateway / LiteLLM
       ↓
     approved models
```

The plugin is integration/UX, **not** the security boundary. Tool and shell execution remain constrained by the external Z3R0 runtime.

### 11.4 VS Code integration

For VS Code coding agents/extensions that support a custom or OpenAI-compatible model endpoint:

```text
VS Code coding agent
       │
       │ custom LLM endpoint
       ▼
Z3R0 AI Gateway / LiteLLM
       │
       ▼
approved model providers
```

A lightweight Z3R0 extension can provide:

- session creation/status;
- workspace/repository discovery;
- current branch context;
- capability/profile display;
- trace/session correlation;
- approval UX where a workflow genuinely requires it.

Again, the extension is not trusted to enforce security.

### 11.5 Local OpenShell runtime

```text
Developer Laptop

IDE / OpenCode / coding agent
        │
        ├── LLM → Z3R0 AI Gateway
        │
        └── tools/shell
                ↓
          OpenShell Gateway / Sandbox
                ├── workspace/filesystem isolation
                ├── process/seccomp boundary
                ├── supervisor network policy
                ├── provider credential substitution
                ├── Z3R0 Phantom middleware
                └── Podman or MicroVM compute
```

OpenShell should host ordinary developer tools rather than Z3R0 replacing them with proprietary commands. `git`, `glab`, `kubectl`, `go`, Docker-compatible CLIs, and other standard tools remain usable through constrained credentials and runtime boundaries.

### 11.6 Automatic profile derivation

Routine sessions should not require the developer to manually approve a long permission questionnaire. Z3R0 should derive most capabilities from context and corporate policy.

Example:

```text
Repository   acme/payments-api
Branch       feature/refund
Project      Go
Profile      go-developer
```

Result:

```text
✓ workspace read/write
✓ Go tooling
✓ feature-branch push
✓ GitLab issue/MR/pipeline read
✓ local container build/run
✓ DEV/QA access according to project scope
✓ PROD diagnostics/read

✗ protected-branch write
✗ PROD mutation
✗ host credentials
✗ arbitrary egress
```

Routine operations should be silent. Frequent confirmation dialogs for edit/test/build/push-feature-branch workflows would train users to bypass the system.

### 11.7 Offline/degraded mode

A cached, signed local policy may continue to permit local operations such as:

```text
edit
test
build
commit
```

Remote authority still requires the relevant online service and short-lived credential flow. Failure of central services must not expand permissions.

---

