---
status: accepted
date: 2026-11-16
deciders: bree
requirements: [Z-INV-2, Z-INV-3]
---
# 0030 — Push gate: the agent can only push its session branch

## Context
The sandbox token can push to every branch its role allows (C43). GitLab tokens are not branch-scoped and OpenShell rules cannot see ref names inside a git push. PoC 11 (#93) showed a small proxy on the host can check them.

## Decision
- `zero up` starts one shared push gate (part of `zero serve`, ADR 0031; 127.0.0.1:18095) for all forge sandboxes. It is always on; there is no off switch for now.
- The sandbox fetches directly from the forge and pushes through the gate (`remote.origin.pushurl`). Direct pushes to the forge are denied by OpenShell rules in the forge profile.
- Allowed: `refs/heads/zero/<name>` or branches below it. Denied: every other branch, tags, deletes, and any push that mixes allowed and denied refs.
- Each sandbox gets a random key in its push URL; the gate maps the key to that sandbox's repository and branch. `zero rm` removes the key.
- The gate holds no credential. The supervisor substitutes the sandbox's token into the request and the gate passes it on.
- Every decision is logged to `<state>/gate.log`.

## Consequences
- One fixed port, because the forge profile is shared by all sandboxes.
- The gate is stateless: a force-push that rewrites the agent's own session branch is not detected.
- `zero/<name>` and `zero/<name>/x` cannot exist together (git ref naming); use one form per sandbox.
- Branch deletion and creation through the GitLab API are closed in #96 (preset without DELETE, token without `create_branch`).
- Not tested with the VM driver.
- The gate becomes a handler of `zero serve` (#97).
