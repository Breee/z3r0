---
status: accepted
date: 2026-11-23
deciders: bree
requirements: [Z-INV-1, Z-INV-2]
---
# 0044 — The agent opens merge requests only where `ci.run` is granted; the push gate refuses push options

Amends ADR 0035.

## Context

ADR 0035 keeps agent-written code off the runners: the push gate adds `ci.skip`, a human starts the pipeline. Tested 2026-11-23 on gitlab.com (#134), default capabilities, no `ci.run`:

- The agent pushed a `.gitlab-ci.yml` with a rule for `merge_request_event` (push pipeline `skipped`), then opened a merge request with its token (`gitlab.mr.create`). GitLab started a merge request pipeline and the agent's job ran on the runner.
- The same with the push option `merge_request.create`. GitLab opens the merge request with push permission alone; the token needs no `create_merge_request`. The merge request pipeline ran although the push carried `ci.skip`.
- A push with `ci.skip` while a merge request is open skips both pipelines. Only the creation of the merge request starts one.

GitLab has no option that opens a merge request without its pipeline.

## Decision

- `gitlab.mr.create` leaves the default capabilities. Default: the agent pushes its session branch, the person opens the merge request.
- A policy that lists `gitlab.mr.create` without `ci.run` is refused at `zero up`. Where `ci.run` is granted, pushes start pipelines anyway and a merge request adds no new path.
- The push gate reads the push options of the agent and refuses the push if there is any. Exception: `merge_request.*` when `gitlab.mr.create` is granted. The option `ci.skip` is still added by the gate itself.

## Consequences

- With the default policy the agent no longer opens merge requests; push output from GitLab shows the link to create one.
- More parser code in the gate facing the agent (option section, same limits as the command section).
- Existing sandboxes: the capability set narrows at the next `zero up`, the token is minted again without `create_merge_request`. A `.zero/policy.yaml` that lists `gitlab.mr.create` without `ci.run` must be changed.
- The option names are GitLab's; other forges need their own check.
- Not covered: a merge request the person opened for the session branch. Later agent pushes carry `ci.skip` and skip its pipeline (tested), but the person starting that pipeline runs the agent's code (ADR 0035, unchanged).
- Checked by `tests/attacks/forge.sh` (REST, push option, MCP `save_merge_request`).
