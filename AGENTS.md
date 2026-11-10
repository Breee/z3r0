# zero — Agent Rules

## HARD RULE: never hack around limitations

If you hit a limitation that may be a bug, a design flaw, or something that would break a security invariant — in OpenShell, podman/docker, glab, gh, opencode, GitLab, or anything else:

1. Stop. Do not work around it (no relaxed policy flags, path tricks, disabled checks, retries that hide the error).
2. Write it down: `ai-docs/caveats.md` entry (C-number) with the exact symptom and versions.
3. Write a submittable bug report: `ai-docs/upstream/<project>-<slug>.md` (upstream issue template, repro, expected vs actual, versions). Check for an existing upstream issue first and link it.
4. Tell the user and let them decide. A documented, intended config option may only be used if the user approves it as a decision (`ai-docs/decisions.md`).

More rules: `ai-docs/AGENTS.md`.
