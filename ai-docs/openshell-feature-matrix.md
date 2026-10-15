# OpenShell Feature Matrix

Every OpenShell feature Z3R0 relies on (arch Appendix A), with test status.
`used` = actively in PoCs/scripts. `tested` = proven working but not in daily use. `not tested` = assumed, not verified.

| Feature | Status | Proof / Caveat |
|---|---|---|
| Sandbox lifecycle (create/exec/upload/delete) | used | F01, all PoCs |
| Non-root workload identity | used | F02, PoC01 |
| Zero Linux capabilities | tested | F02 containment probe |
| `no_new_privs` | tested | F02, C12 (setuid neutralised) |
| Landlock filesystem restrictions | tested | F02 containment probe |
| Seccomp syscall restrictions | tested | F02 containment probe |
| Supervisor-mediated network/DNS | used | F02, PoC03, PoC04 |
| Network policy (endpoint allow/deny, binary-scoped) | used | PoC04 (opencode only), PoC05 (copilot only) |
| Policy update (dynamic, `--rule-name`, `--add-endpoint`) | used | PoC04, PoC05 |
| Policy advisor (`rule get\|approve\|reject`) | not tested | notes only |
| Provider placeholders + endpoint-bound substitution | used | PoC03, PoC04 attack |
| Placeholder resolution (query, header, basic auth) | tested | C17, launder:* findings |
| Foreign-key pass-through (no header matchers) | tested | C18, PoC03 foreign-* FAIL |
| Synthetic DNS (198.18.0.0/15) | tested | C16 |
| OCSF runtime/security observability | not tested | no PoC |
| Inference routing | not tested | no PoC |
| Rule approve workflow | not tested | notes only |
| Podman driver (rootless) | used | F01, all PoCs |
| MicroVM driver | not tested | F16 deferred |
| Kubernetes driver | not tested | Phase 3 |
| `sandbox connect --editor vscode` | used | PoC05 |
| `sandbox ssh-config` | used | PoC05 |
| `sandbox upload` / `sandbox download` | used | C03, C10, C20, C21 |
| cgroup limits (CPU/mem/pids) | tested | P08: host safe, sandbox dies |
| Disk quota | not tested | C22, no quota found |
