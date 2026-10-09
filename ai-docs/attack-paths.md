# Attack paths found

Proven or confirmed paths that are NOT blocked. Source: F02, F03, F08 runs; details in `caveats.md` (C) and F07 `catalog.md`.
Sorted by severity. "Proof" = how to reproduce.

| # | Severity | Path | Impact | Proof | Ref | Mitigation |
|---|---|---|---|---|---|---|
| P01 | high | Sandbox writes `.git/config` (core.fsmonitor, hooksPath, sshCommand, pager…) → user runs `sandbox download` → `git status` on host | Code exec as host user → ~/.ssh, podman, gateway secrets | `tests/attacks/run.sh` dl:git-fsmonitor-host-exec | C20, X01 | Never copy raw `.git` back; return work via git push/MR or patch |
| P02 | high | Agent sends its own/attacker API key (Authorization, X-Api-Key, ?api_key) to an allowed endpoint | Data exfil into attacker account on a legit service (e.g. LLM provider) | `pocs/03-credential-placeholder/run.sh` foreign-* FAIL | C18, X10 | Query deny rules; upstream request (`upstream/`); own egress gateway (F10) |
| P03 | high | Sandbox breakout (kernel/container CVE) → host user `bree` | gateway db + KEK = all provider secrets; ssh key+agent, gh/glab/docker tokens, podman socket; docker group = host root | `tests/breakout/blast-radius.sh` (14 findings) | C13, C25, A18, C24 | Gateway + podman as dedicated user (D06, F09); VM driver; short-lived creds |
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

## Mitigable with OpenShell 0.1.2 alone?

yes = OpenShell policy/config closes it · partial = narrows it · no = needs host setup, workflow, or upstream change.

| # | OpenShell today | How / why not |
|---|---|---|
| P01 | no | `sandbox download` copies `.git` as-is. Workflow fix: push from sandbox, never download repos. |
| P02 | partial | Query deny rules block `?api_key=`-style keys. No header matchers → foreign Authorization/X-Api-Key pass (upstream request / F10). |
| P03 | no | Gateway runs as login user; KEK next to db. Host fix: dedicated user (F09 step 2). VM driver unknown (F09 step 4). |
| P04 | partial | Tight method/path/query rules on bound endpoints shrink it; placeholder still resolved in any header/basic auth, responses not redacted. |
| P05 | no | Download keeps absolute symlinks. Workflow fix: don't download. |
| P06 | no | No disk limit found in sandbox settings. Host fix: quota on podman storage (F09). |
| P07 | partial | Few endpoints, read-only, path/query rules. Never fully closable. |
| P08 | partial | cgroup limits protect host (default 4 GiB/2048 pids/2 CPU); sandbox still dies. |
| P09 | no | Mount/hosts info comes from podman; kernel shared. |
| P10 | yes | Don't allow `tls: skip` or non-HTTP endpoints in policy. |
| P11 | no | Image trust is podman `policy.json` (C08), not OpenShell. |
| P12 | no | Use unique sandbox names (workflow). |

Unverified: "no" for P06/P09 means not found in 0.1.2 docs/CLI, not proven impossible.
