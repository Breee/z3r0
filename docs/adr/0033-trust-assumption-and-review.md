---
status: accepted
date: 2026-11-16
deciders: bree
requirements: [Z-INV-1, Z-INV-3, Z-INV-5]
---
# 0033 — Trust assumption: sandbox and supervisor are trusted; review in the MR or the remote view

## Context
A corrupted agent can reach the developer's account on the laptop by five routes:

1. The developer runs agent-committed code on the host (hooks, `Makefile`, tests, `.envrc`, editor tasks; C42).
2. A bug in `zero serve` (push gate), which runs as the login user and parses bytes sent by the agent.
3. A bug in a host client that talks to the sandbox (opencode web UI/TUI, VS Code Remote-SSH; P18).
4. A bug in the OpenShell supervisor, which holds the real secrets.
5. A sandbox escape (kernel; with `--vm` also the hypervisor).

zero cannot defend all five itself. It has to say which it relies on and which it works on.

## Decision
- The OpenShell sandbox and supervisor are the trust assumption. Routes 4 and 5 are accepted; zero keeps OpenShell pinned and current, and offers the VM driver (`zero up --vm`) as the stronger boundary.
- Routes 1–3 are zero's job:
  - Route 1: code from an agent branch is run only inside a sandbox (`zero up` on that branch), never on the host. zero does not enforce this; it is the documented rule.
  - Route 2: `zero serve` is hardened (#100).
  - Route 3: not zero's code; documented recommendation only.
- Review needs no host checkout. Two paths, both supported:
  - The agent pushes its session branch through the push gate, the MR is reviewed in the forge.
  - The developer watches the changes in VS Code Remote or the opencode web UI and commits and pushes from inside the sandbox.
- Every push from a sandbox is accountable to the developer who started it. zero does not tell agent and developer apart inside the sandbox.

## Consequences
- A supervisor bug or sandbox escape is outside what zero claims to contain.
- A developer who checks out an agent branch on the host and runs its tooling is unprotected (C42 stays open as a documented risk).
- CI runs agent code, including a changed pipeline file (P14); runner isolation and protected CI variables are the forge's side (ADR 0024).
- Removing `git.push` from the project policy blocks the developer's pushes from the sandbox as well as the agent's.
