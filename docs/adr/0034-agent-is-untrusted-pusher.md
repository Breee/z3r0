---
status: accepted
date: 2026-11-16
deciders: bree
requirements: [Z-INV-1, Z-INV-2, Z-INV-5]
---
# 0034 — The agent is an untrusted pusher, not a regular developer

## Context

The agent pushes with a token of the developer, so the forge sees a developer. Forge defaults trust developers: any of them may change the pipeline, and a push runs it. A human who ships all CI variables to a remote server is unlikely and accountable. A corrupted agent does it immediately. CI is the one place where agent code runs outside the sandbox without a human step (attack path P14).

## Decision

- Whatever the agent pushes is untrusted input for everything that runs it automatically (CI, bots, webhooks), like a merge request from an unknown fork.
- Accountability stays with the developer (ADR 0020, 0033); trust does not follow from it.
- Controls that are sufficient for humans are not assumed sufficient for the agent. Each automatic consumer of agent pushes needs its own layer or a recorded acceptance.
- Mitigation for CI is not decided here; candidates are listed in `ai-docs/attack-paths.md` (P14 in depth) and tracked in #102.

## Consequences

- Today no zero layer covers P14. Projects with unprotected CI variables or self-hosted runners are exposed as soon as the agent may push.
- Part of the fix is forge configuration (protected variables, runner scope), which zero can check but not enforce (ADR 0024).
- A layer that delays pipelines until a human looked costs convenience: the agent cannot see CI results on its own.
