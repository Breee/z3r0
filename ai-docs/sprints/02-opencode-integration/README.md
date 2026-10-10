# Sprint 02 — OpenCode × OpenShell integration

Goal: `zero up` in a repo → OpenCode session whose every tool runs inside a persistent per-repo OpenShell sandbox; host TUI/VS Code attach.
Exit: daily usable loop, proven by review.md; no host-side tool execution.

| ID | Feature | Ref | Status |
|---|---|---|---|
| F11 | Dev image: golang + git + opencode, pinned digest | D08, C08, C09, C12 | done |
| F17 | `zero up/attach/down`: host TUI attaches to `opencode serve` inside | D08, D09 | done |

Order: F11 → F17.

Not doing: host-side bash-only plugins (workspace diverges, read/edit on host — F04 Option B). Two-way sync (copies .git back, P01).
