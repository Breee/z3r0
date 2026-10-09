# Sprint 02 — Usable + safer dev loop

Goal: `z3r0 up` gives a persistent per-repo sandbox with OpenCode/VS Code inside; code leaves only via git push; high gaps closed.
Exit: one-command loop works daily; all high paths in attack-paths.md mitigated or accepted with a decision.

| ID | Feature | Gap | Status |
|---|---|---|---|
| F11 | Dev image: golang + git + opencode, pinned digest | D08, C08, C09 | todo |
| F17 | `z3r0 up/attach/down`: per-repo sandbox, host TUI attaches to `opencode serve` inside | D08, D09, convenience | todo |
| F12 | Push-from-sandbox workflow, never `sandbox download` | P01, P05 (C20, C21) | todo |
| F13 | Egress/LLM proxy strips foreign keys (was F10) | P02, P04 (C17, C18, C27) | todo |
| F14 | Short-lived GitLab creds + real-token test (was F03 rest) | F03, C26 | todo |
| F15 | Host hardening: KEK, client key, docker group, disk quota | P03, P06 (C13, C22, C25, C26) | todo |
| F16 | MicroVM driver trial | P03, P09, C13 | todo |

Order: F11 → F17 → F12 → F13 → F14 → F15 → F16.

Not doing: host-side bash-only plugins (opencode-sandbox, container-exec) — workspace copy diverges, read/edit stay on host (F04 Option B gap). Two-way sync (agentbox/mutagen) — copies .git back (P01).
