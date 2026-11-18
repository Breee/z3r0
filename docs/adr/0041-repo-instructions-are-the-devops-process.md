---
status: accepted
date: 2026-11-18
deciders: bree
requirements: [Z-INV-1, Z-INV-5]
---
# 0041 — Instructions in the repo are the DevOps process's job, not zero's

## Context
P27 (#109): opencode in the sandbox loads `AGENTS.md`, `opencode.json` (MCP servers, `permission`, plugins), and `.opencode/` (plugins, tools, agents, commands, skills) from the checkout. zero clones the host repo's `HEAD` (`zero up`, bundle + clone), so the agent starts on whatever branch the developer has checked out. That can be an unreviewed MR branch, whose instructions and plugins then run with the agent's rights.

#109 proposed to take these files from the protected branch instead.

## Decision
- zero does not filter, pin, or replace repo instructions or repo opencode config. What a repo tells its agent is the project's content. MR review and branch protection (the DevOps process) decide it, as they do for code, CI config, and build scripts.
- zero still enforces its own boundary, whatever the repo config says: egress allowlist, no secrets in the sandbox, push gate limited to `zero/<sandbox>` and branches below it, no deletes; default-branch protection checked by `zero doctor`.
- P27 is accepted.

## Consequences
- An agent started on a branch with poisoned instructions follows them within the sandbox's limits. Choosing the branch is the developer's decision.
- Changes the agent makes to its own config stay on its session branch and reach the default branch only through a reviewed MR.
- No separate config source and no "edit agent config" workflow in zero.
- Rejected: copy instruction paths from the default branch at `zero up` (#109). That duplicates review that belongs in the forge, and hides the repo's actual config from the developer working on it.
