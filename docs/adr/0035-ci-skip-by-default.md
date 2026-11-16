---
status: accepted
date: 2026-11-16
deciders: bree
requirements: [Z-INV-1, Z-INV-2]
---
# 0035 — Agent pushes start no pipeline by default (`ci.skip`); `ci.run` opts in

## Context

A push to the session branch starts CI, which runs agent-written code outside the sandbox (P14, ADR 0034). Tested on gitlab.com (#102): the job read an unprotected CI variable, had a job token and open internet on a self-hosted runner. The agent token cannot start pipelines itself.

## Decision

- The push gate adds the push option `ci.skip` to every push it forwards. The forge records the pipeline as skipped; a human starts it from the MR or pipeline page after reading the diff.
- Risk tolerance is a project setting: capability `ci.run` in `.zero/policy.yaml` turns the option off, so pipelines run on push. It is not in the default set and adding it is a policy widening (approval prompt, #78).
- `zero doctor` fails on project CI variables that are not protected.

## Consequences

- The gate now rewrites the command section of the push (adds the `push-options` capability and the option). More parser code facing the agent (hardening: #100).
- A started pipeline still runs agent code on the runner with the runner's network. The human step is the control; protected variables and protected runners stay recommended.
- `ci.skip` is a GitLab option name; other forges need their own.
- Not built: check of inherited group variables and runner scope in `zero doctor`.
- "Merge only with green pipeline" means a human must start the pipeline before merging.
