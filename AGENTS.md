# zero — Agent Rules

## HARD RULE: never hack around limitations

If you hit a limitation that may be a bug, a design flaw, or something that would break a security invariant — in OpenShell, podman/docker, glab, gh, opencode, GitLab, or anything else:

1. Stop. Do not work around it (no relaxed policy flags, path tricks, disabled checks, retries that hide the error).
2. Write it down: `ai-docs/caveats.md` entry (C-number) with the exact symptom and versions.
3. Write a submittable bug report: `ai-docs/upstream/<project>-<slug>.md` (upstream issue template, repro, expected vs actual, versions). Check for an existing upstream issue first and link it.
4. Tell the user and let them decide. A documented, intended config option may only be used if the user approves it as a decision (new ADR in `docs/adr/`).

## HARD RULE: project management

GitHub issues + milestones (Breee/z3r0) are the source of truth and must be up to date at all times.

1. Every work item has an issue before work starts: milestone (phase/sprint) + `type:` label. No untracked work.
2. Every issue states **GOOD** (the target behaviour/quality bar) and **DONE** (checkable exit criteria) in its body.
3. Commits reference the issue: `<what> (#N)`.
4. Closing requires **PROOF**: a comment with commit(s) and the exact commands run + decisive output that show each DONE criterion met.
5. Closing requires **REVIEW**: a subagent reviews the change/result against GOOD/DONE before closing; findings and their resolution go into the closing comment.
6. Status changes (blocked, postponed, moved, not planned) are reflected immediately: label, milestone, comment with reason.

More rules: `ai-docs/AGENTS.md`.
