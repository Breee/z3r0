# ai-docs — Agent Rules

Planning + tracking for Z3R0. Source docs: `../Z3R0_10min_summary.md`, `../Z3R0_ARCHITECTURE_v0.9_OPEN_SHELL.md`.

## Layout

```
ai-docs/
├── AGENTS.md                 # this file
├── decisions.md              # ADR log (append-only)
├── caveats.md                # known gaps, risks, gotchas
└── sprints/NN-<slug>/
    ├── README.md             # sprint goal, feature list, status table
    └── FNN-<slug>.md         # one file per feature
```

## Rules

- Work in sprints. One sprint = one directory. One feature = one file.
- Keep everything short. No fluff. Bullets over prose.
- Update feature `Status` + `Log` on every meaningful change.
- New decision → append to `decisions.md`. New gap/risk → append to `caveats.md`.
- Feature is DONE only when every `Proof` step passed and its result is logged.
- Do not rebuild OpenShell/GitLab/K8s. Evaluate and compose.

## Feature template

```
# FNN — <title>
Status: todo | doing | blocked | done
Description: <1–2 lines, what + why>
Goal: <1 line, outcome>
Plan:
1. <step>
DONE: <checklist of objective completion criteria>
GOOD: <quality bar beyond DONE>
Proof: <commands/tests that show it works + expected result>
Log:
- YYYY-MM-DD: <what happened>
```

## Status values

`todo` → `doing` → `done`; `blocked` needs a reason in Log.
