# Z3R0 — Short Architecture Summary

## Problem statement

AI coding and operations agents are useful because they can act autonomously, but they also process untrusted input: issues, documentation, logs, tool responses, dependencies and source code.

A normally benign agent can therefore become **temporarily corrupted** by prompt injection, malicious tooling or supply-chain content.

The important question is not:

> Can we make the model always detect malicious instructions?

It is:

> **What can a fully corrupted agent still do?**

Z3R0 aims to make the answer: **only what was explicitly allowed, inside a contained environment, without ever receiving reusable secrets or direct production authority.**

## What needs to be solved

### 1. Isolation
Agent-controlled code, tests, scripts and container builds must run inside a strong sandbox.

**OpenShell** provides the execution boundary:

- filesystem/process isolation;
- restricted network access;
- Podman / MicroVM / Kubernetes execution;
- policy enforcement.

### 2. Credential replacement
Neither agents nor developers should receive real reusable secrets.

The **Phantom Proxy** mediates authenticated access:

```text
agent → placeholder credential → Phantom Proxy → real credential → target
```

It also sanitizes sensitive response data before it reaches the sandbox.

### 3. DevOps/GitOps workflows stay authoritative
Z3R0 does not invent a separate AI delivery process.

Agents are treated like developers and use existing controls:

- feature branches;
- protected `main`;
- Merge Requests;
- CI/CD;
- Kubernetes RBAC/admission;
- GitOps with Argo CD / Flux;
- Cilium/Tetragon;
- LiteLLM for models/MCP/tools.

### 4. Production is not writable
In v1, normal agents do **not mutate production**.

They work in DEV/QA. Production access is limited to explicitly approved, sanitized diagnostic reads such as logs and selected resource specs.

### 5. Explicit capabilities and human approval
Agents receive scoped capabilities, not ambient authority.

Examples:

```text
git.push feature/*
gitlab.pipeline.read
k8s.logs.read
container.build
```

Out-of-scope operations require the agent to explain what it wants to do and why. A human approves or rejects the concrete request; sensitive exceptions are one-shot.

### 6. Audit and analysis
Every relevant action must be attributable to:

- the human;
- the logical agent session;
- the current security epoch/runtime;
- the capability used;
- the target resource;
- the policy/approval decision;
- the result.

This allows Z3R0 to answer:

> **Who asked the agent to do what, what authority was used, what actually happened, and was anything suspicious?**

Organization context such as service maps, criticality and dependencies can additionally **escalate risk**, but never lower a deterministic security decision.

## Simple example

A Go developer asks an agent to implement a feature:

```text
Developer
   ↓
OpenCode / VS Code
   ↓
Z3R0 session
   ↓
OpenShell sandbox
   ↓
edit code → go test → build container
   ↓
push feature branch
   ↓
GitLab CI
   ↓
create MR
```

The agent may edit the isolated workspace, run tests/builds, inspect CI and push its feature branch.

It may **not** read host secrets, obtain real GitLab/Kubernetes credentials, push `main`, bypass CI/GitOps or mutate production.

## Core idea

> **OpenShell secures where the agent runs. Z3R0 controls what authority it receives and forces agent work through established DevOps/GitOps practices.**
