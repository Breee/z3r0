# Sprint 03 — Hardening

Goal: close Sprint 01 high gaps.
Exit: all high paths in attack-paths.md mitigated or accepted with a decision.

| ID | Feature | Gap | Status |
|---|---|---|---|
| F12 | Push-from-sandbox workflow, never `sandbox download` | P01, P05 (C20, C21) | todo |
| F13 | Egress/LLM proxy strips foreign keys (was F10) | P02, P04 (C17, C18, C27) | todo |
| F14 | Short-lived GitLab creds + real-token test (was F03 rest) | F03, C26 | todo |
| F15 | Host hardening: KEK, client key, docker group, disk quota | P03, P06 (C13, C22, C25, C26) | todo |
| F16 | MicroVM driver trial | P03, P09, C13 | todo |

Order: F12 → F13 → F14 → F15 → F16.
