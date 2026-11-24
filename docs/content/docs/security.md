---
title: Security
weight: 4
description: What the agent can do, what it cannot do and what is still open.
---

zero assumes that the agent can be turned against you: by a prompt hidden in an issue, by a dependency, by a mistake of the model. So nothing depends on the agent behaving well. Every check runs outside the sandbox.

## What the agent has

- a copy of the repositories and input folders of its workspace
- network access to the workspace's projects on the GitLab host (API and git fetch, nothing else on that host), the model endpoint and the hosts in the [policy](../policy)
- a GitLab token for exactly the projects of the workspace, with the permissions of the policy, valid for one to two days (it ends at midnight UTC of the day after tomorrow) and renewed while the sandbox runs
- one way to hand over work: `git push` to `zero/<workspace>`

## What it cannot do

| The agent cannot | Because |
|---|---|
| read your home directory, SSH keys or other repositories | it runs in an OpenShell sandbox with its own file system |
| read its own GitLab token | the sandbox holds a placeholder; the real token is put into the request outside the sandbox |
| reach a host that is not allowed | all traffic passes the sandbox proxy; other hosts are blocked |
| push to `main`, another branch or a tag, or delete a branch | the push gate allows `zero/<workspace>` and branches below it only; a direct push to GitLab is blocked |
| start a pipeline with its code | the push gate adds `ci.skip` and refuses push options, unless the policy grants `ci.run` |
| open a merge request | the token has no permission for it, unless the policy grants `gitlab.mr.create` |
| widen its own policy | the policy is read from your host checkout and a wider one needs your approval |
| keep access after you stop | `zero pause` and `zero rm` revoke the token |

These points are tested against a running sandbox by [`tests/attacks/forge.sh`](https://github.com/Breee/z3r0/blob/main/tests/attacks/forge.sh). Every decision of the push gate is written to `~/.local/state/zero/zero-<name>/gate.log`. Requests the gate refuses are written there too, and to the journal of `zero-cp` (`journalctl --user -u zero-cp`). One push may be 2 GiB at most, may pause for two minutes at most and takes 30 minutes at most; a sandbox has two pushes at a time.

Text that `zero` prints from a sandbox (unpushed work, notes, error messages, `zero logs`) has its control characters removed, so the agent cannot repaint your terminal. The opencode screen of `zero opencode` and the answers of `zero run` come from opencode inside the sandbox, like any ssh session, and are not filtered by `zero`: while such a command runs, a program in the sandbox can write to your terminal.

## What you have to do

- **Review the branch before you run anything from it.** Code of the agent runs with your rights as soon as you check out its branch and run tools on it: hooks, `Makefile`, `.envrc`, editor tasks, tests.
- **Protect the default branch in GitLab.** No direct push, no force push, merge only with a green pipeline, protected CI variables. `zero doctor` reports what is missing.
- **Grant `ci.run` only where the runners can take it.** With it, tests written by the agent run on your runners.

## Known open points

| Open point | Effect | Tracked |
|---|---|---|
| Data can leave through an allowed host | the agent can send data to an allowed host under an account of its own; keep `network.allow` short | [#37](https://github.com/Breee/z3r0/issues/37) |
| Text from outside reaches the agent unmarked | an issue or merge request comment of a stranger can carry instructions for the agent | [#110](https://github.com/Breee/z3r0/issues/110) |
| The agent can force-push its own branch | history of `zero/<workspace>` can be rewritten; other branches are not affected | [attack paths](https://github.com/Breee/z3r0/blob/main/docs/attack-paths.md) |
| `ci.run` together with `gitlab.mr.create` | a merge request opened by the agent through the API runs a changed pipeline definition | [ADR 0045](https://github.com/Breee/z3r0/blob/main/docs/adr/0045-ci-run-skips-while-the-branch-changes-the-pipeline-definition.md) |
| A foreign token can be sent to the workspace's own projects | the GitLab host is reachable for the workspace's projects only, but OpenShell cannot tell whose token a request carries | [ADR 0046](https://github.com/Breee/z3r0/blob/main/docs/adr/0046-forge-egress-limited-to-the-workspaces-projects.md) |
| No GitLab MCP tools in the sandbox | the MCP endpoint is not reachable from the sandbox; it comes back with the MCP gateway, which is not built in yet | [ADR 0043](https://github.com/Breee/z3r0/blob/main/docs/adr/0043-mcp-through-shared-agentgateway.md) |

## Background

- [Core security invariants](https://github.com/Breee/z3r0/blob/main/docs/architecture/02-core-security-invariants.md)
- [Attack paths](https://github.com/Breee/z3r0/blob/main/docs/attack-paths.md)
- [ADR 0030: push gate](https://github.com/Breee/z3r0/blob/main/docs/adr/0030-push-gate.md)
- [Known limitations](https://github.com/Breee/z3r0/blob/main/ai-docs/caveats.md)
