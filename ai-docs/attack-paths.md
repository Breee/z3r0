# Attack paths found

Proven or confirmed paths that are NOT blocked. Source: F02, F03, F08 runs; details in `caveats.md` (C) and F07 `catalog.md`.
Sorted by severity. "Proof" = how to reproduce.

| # | Severity | Path | Impact | Proof | Ref | Mitigation |
|---|---|---|---|---|---|---|
| P01 | high | Sandbox writes `.git/config` (core.fsmonitor, hooksPath, sshCommand, pager…) → user runs `sandbox download` → `git status` on host | Code exec as host user → ~/.ssh, podman, gateway secrets | `tests/attacks/run.sh` dl:git-fsmonitor-host-exec | C20, X01 | Never copy raw `.git` back; return work via git push/MR or patch |
| P02 | high | Agent sends its own/attacker API key (Authorization, X-Api-Key, ?api_key) to an allowed endpoint | Data exfil into attacker account on a legit service (e.g. LLM provider) | `pocs/03-credential-placeholder/run.sh` foreign-* FAIL | C18, X10 | Query deny rules; upstream request (`upstream/`); own egress gateway (F10) |
| P03 | high | Sandbox breakout (kernel/container CVE) → host user `bree` | Reads gateway db + KEK next to it = all provider secrets; podman socket; ~/.ssh | file ownership evidence (F02); measured in F09 | C13, C18(cat), A18, C24 | Gateway + podman as dedicated user (D06, F09); VM driver; short-lived creds |
| P04 | medium | Placeholder laundering: agent puts placeholder in query / any other header / basic auth on the bound endpoint; upstream echoes or stores it | Real secret returned to agent | `tests/attacks/run.sh` launder:* GAP | C17, X04 | Never bind creds to reflecting endpoints; tight path/query rules |
| P05 | medium | Absolute symlink in workspace (e.g. → ~/.ssh) survives `sandbox download` | Host tools following the link read/write host files | dl:abs-symlink-kept GAP | C21, X02 | Don't download; or strip symlinks after download |
| P06 | medium | Disk fill: no quota on sandbox overlay/workspace | Host disk full (DoS) | res:no-disk-quota GAP | C22, C12(cat) | Quota on podman storage of gateway user (F09) |
| P07 | inherent | Data in query/path/body of any allowed endpoint | Source/arch exfil | exfil:data-in-query GAP | C19, A11, X03 | Few endpoints, read-only, tight path/query rules, Z3R0 audit |
| P08 | low | Memory hog → sandbox OOM-killed, no restart | Work in sandbox lost (host safe) | res:oom-kills-sandbox GAP | C23 | Push work early |
| P09 | low | Info leaks: /proc/mounts + mountinfo (username, host paths), /etc/hosts (hostname, LAN IP), kernel version | Recon for P03 | `tests/containment/run.sh`, info:* | C11, C24 | Patch kernel; VM driver |
| P10 | low | Non-HTTP / `tls: skip` endpoints: no L7 check, no credential rewrite | Uninspected channel if such endpoint is allowed | docs only | C15, C19(cat) | Don't allow such endpoints |
| P11 | low | Unsigned image from an allowed registry | Malicious image runs as sandbox | policy.json review | C08, C17(cat) | sigstore/GPG verification |
| P12 | low | Same-name sandbox recreate right after delete | Stale state / failed run | observed in F02 | C14, X07 | Unique names |

Open (not tested yet): prompt injection via tool output (A07, F04), social-engineered rule approval (X06, F04), VS Code server phoning home (X08, F05), VM driver (C13, F09).
