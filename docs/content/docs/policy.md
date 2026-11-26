---
title: Policy
weight: 3
description: Hosts, capabilities and tools of a workspace.
---

The policy says which hosts the sandbox reaches and what the agent's token may do. It is a file in the repository:

```yaml {filename=".zero/policy.yaml"}
network:
  allow:
    - proxy.golang.org
    - sum.golang.org

capabilities:
  - git.read
  - git.commit
  - git.push
  - gitlab.issue.read
  - gitlab.mr.read
  - gitlab.pipeline.read
```

Without the file a project gets the capabilities above and no extra host. The GitLab host and the model endpoint are always reachable.

## Who can change it

zero reads the file from your checkout on the host, never from the sandbox. The agent can edit its copy and push it. That has no effect until you check out its branch on the host and run `zero up`.

When the file allows more than before (a new host or capability), `zero up` prints what is new and asks:

```
policy widens for zero-developer:
  + network example.com
approve? [y/N]
```

`zero up --approve` prints the same lines and answers yes. Without a terminal and without `--approve`, `zero up` refuses. A policy that allows less is applied without a question. An unknown key or capability stops `zero up`; zero never falls back to a wider policy.

A workspace with several repositories gets the union of their policies.

## Hosts

Each entry under `network.allow` opens HTTPS (port 443) to that host, read-only. Everything else is blocked.

A change to the file takes effect at the next `zero up`, also on a running sandbox: a new host is reachable after the approval, a removed host is refused from then on.

## Capabilities

| Capability | Default | What the agent can do |
|---|---|---|
| `git.read` | yes | read and clone the projects of the workspace |
| `git.commit` | yes | commit in the sandbox |
| `git.push` | yes | push to `zero/<workspace>` through the push gate |
| `gitlab.issue.read` | yes | read issues |
| `gitlab.mr.read` | yes | read merge requests |
| `gitlab.pipeline.read` | yes | read pipelines and jobs |
| `ci.run` | no | pushes may start a pipeline |
| `gitlab.mr.create` | no | open merge requests; only together with `ci.run` |

### ci.run

Without `ci.run` the push gate adds `ci.skip` to every push, so no pipeline runs agent code on your runners.

With `ci.run` a push starts the pipeline, with one exception: while the agent's branch changes a pipeline definition, the pipeline is still skipped and the push output says so. A human starts it after a look at the change. Pipeline definition means the project's CI configuration file (default `.gitlab-ci.yml`), everything under `.gitlab/` and every `.yml` or `.yaml` file.

`ci.run` also gives the token API read access to the whole repository of the project.

### gitlab.mr.create

A merge request starts a merge request pipeline, and GitLab has no way to skip it. That is why this capability is refused without `ci.run`:

```
.zero/policy.yaml: gitlab.mr.create needs ci.run: a merge request opened by the agent starts a pipeline; add ci.run or take gitlab.mr.create out
```

With the default policy you open the merge request yourself.

## Tools

The GitLab MCP tools of a workspace are set in zero's own configuration, not in the repository:

```yaml {filename="~/.config/zero/config.yaml"}
workspaces:
  team:
    folders: [...]
    tools: [default]
  small:
    folders: [...]
    tools: [get_project, list_branches]
```

`default` stands for 15 read tools (projects, files, branches, tags, commits, merge requests, pipelines, jobs, work items, labels). The token gets the permissions these tools need on top of the capabilities. A changed list takes effect at the next `zero up`. An unknown tool name stops every zero command until it is corrected.

{{< callout type="warning" >}}
In this version the list decides the token's permissions only. The sandbox cannot reach GitLab's MCP endpoint, so the agent has no GitLab MCP tools until the MCP gateway is built in.
{{< /callout >}}

## Instructions and skills

Instructions (text files the agent always reads) and skills (folders with a `SKILL.md`) of a workspace are set in the same file:

```yaml {filename="~/.config/zero/config.yaml"}
workspaces:
  team:
    folders: [...]
    instructions: [~/zero/instructions/review.md]
    skills: [~/zero/skills/go-review]
```

`zero up` copies them into the sandbox and tells opencode about them:

```text
instructions: review.md, read-only
skills: go-review, read-only
```

The agent can read them and cannot change them. A changed or removed entry takes effect at the next `zero up`; `zero status` shows both lists. A path that does not exist, a skill folder without `SKILL.md` or two entries with the same name stop every zero command until corrected.

Limits:

- Only opencode is told about them. Other tools in the sandbox find the files under `/tmp/zero-input/.zero/` and nothing more.
- A skill is offered to the model only if its `SKILL.md` has a `description`.
- They count into the size limit of input folders (`zero up --max-input`), need the folder `zero install` creates, and do not work with `--vm`.
- What a repository sets in its own `opencode.json` is added to them, not replaced.
- If the sandbox already has an opencode user config that zero did not write, zero leaves it alone and says so.

## Background

- [Policy source and lifecycle](https://github.com/Breee/z3r0/blob/main/docs/architecture/07a-policy-sources-and-lifecycle.md)
- [ADR 0035: pushes skip the pipeline](https://github.com/Breee/z3r0/blob/main/docs/adr/0035-ci-skip-by-default.md)
- [ADR 0044: merge request creation needs ci.run](https://github.com/Breee/z3r0/blob/main/docs/adr/0044-merge-request-creation-needs-ci-run.md)
- [ADR 0045: ci.run skips while the branch changes the pipeline definition](https://github.com/Breee/z3r0/blob/main/docs/adr/0045-ci-run-skips-while-the-branch-changes-the-pipeline-definition.md)
