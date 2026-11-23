---
status: accepted
date: 2026-11-23
deciders: bree
requirements: [Z-INV-1, Z-INV-2, Z-INV-3]
---
# 0045 — With `ci.run` the push gate skips the pipeline while the session branch changes a pipeline definition file

Amends ADR 0035.

## Context

With `ci.run` the push gate added no `ci.skip`: every agent push started a pipeline, also one whose definition the agent had just written (#126). The person granting `ci.run` wants the project's pipeline to run on agent code, not a pipeline of the agent's design (other runner tags, other variables, deploy jobs).

The gate passes the pack through unparsed and does not see file names. Two ways were weighed: unpack the agent's pack on the host with git (parser on agent data on the host, git needed next to zero-cp), or let the forge do it.

## Decision

- The forge does it. For a push with `ci.run` the gate pushes the agent's pack to a temporary branch `zero-check/<name>-<random>` with `ci.skip`, asks the forge's compare API which files the pushed commit changes against the default branch (from their merge base), then updates the session branch with a pack without objects, with or without `ci.skip`, and deletes the temporary branch.
- The pipeline is skipped while the comparison lists a definition file: the project's CI config file (`ci_config_path`, default `.gitlab-ci.yml`), anything under `.gitlab/`, and every `.yml`/`.yaml` file. Every YAML file, because a local include can sit anywhere and must be one; the gate does not read the definition to follow includes.
- The whole branch is compared, not the pushed commits: a later push that leaves the definition alone is skipped too, until the branch no longer differs.
- Anything unclear skips the pipeline: more than one ref in the push, no token in the request, the forge refusing or timing out, 300 or more changed files (gitlab.com listed 633 files in full and cut at 3000 with `compare_timeout` set, tested 2026-11-23; the margin is for instances set lower). A pack the forge refuses on the temporary branch is refused to the agent with the forge's reason.
- The push output tells the agent: `remote: zero: pipeline skipped: <reason>`. The reason is in `gate.log`.
- A push that is skipped and carries a push option (`merge_request.*`, allowed with `gitlab.mr.create`) is refused: the merge request would start its own pipeline with the changed definition.
- `ci.run` adds the token permission `read_repository` (the compare API needs it). The agent's token can then read every branch, commit and archive of the project by API; `git.read` gives the same content through git.

## Consequences

- Each push with `ci.run` costs four requests to the forge more (temporary push, project, comparison, delete), and leaves a pipeline record `skipped` for the temporary branch. Pushes without `ci.run` are unchanged.
- A repository that is mostly YAML (GitOps) rarely gets a pipeline from an agent push; a person starts it. A narrower list per project is possible later, not built.
- The temporary branch is outside `zero/<name>` (git cannot hold a branch below an existing branch). The agent cannot push it; a branch rule on `zero-check/*` that forbids pushes breaks the check, and every push is then refused.
- A temporary branch can stay behind if the delete fails; it is logged in `gate.log`.
- Compared from the merge base: the definition the pipeline starts from was on the default branch at some time, not necessarily its newest state.
- Not covered: scripts, Makefiles and tests the jobs call. They run with `ci.run`; that is what the capability grants (ADR 0035).
- Open: with `gitlab.mr.create` the agent opens a merge request by API; the gate does not see that call and the merge request pipeline runs the changed definition. Who needs this check to hold does not grant `gitlab.mr.create`.
- Sandboxes that have `ci.run` already: until the token is minted again the comparison is refused and every push is skipped.
- The compare call and the file rule are GitLab's; another forge needs its own.
- Checked by `tests/attacks/forge.sh` on a sandbox with `ci.run`.
