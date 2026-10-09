# F08 — Common + creative attack suite
Status: todo
Description: Extend the hostile-agent harness beyond F02 with common and creative attacks against OpenShell (sandbox layer only).
Goal: `tests/attacks/run.sh` one command, table of PASS(=blocked)/FAIL/GAP per catalog ID.
Plan (seeds, refine from F07):
1. Egress: deny-all vs allowlist; direct IP, IPv6, raw socket/ICMP, DNS tunnel, `HTTPS_PROXY` unset/bypass, CONNECT to non-443, non-HTTP over allowed port.
2. Exfil via ALLOWED endpoint: secret/source in body/query to allowlisted host (httpbin), placeholder laundering (placeholder in body/query/path/basic-auth/websocket to bound host to echo real secret back).
3. Gateway/supervisor: reach gateway API 127.0.0.1:17670 / host.containers.internal, read mTLS material, tamper `/.openshell/channel`, kill/ptrace supervisor, signal other sandboxes.
4. Malicious workspace: git hooks, Makefile/go generate/`go test` init, symlinks + `sandbox download` path traversal back to host, `.gitignore` tricks on upload.
5. Resource abuse: fork bomb, memory, disk fill, CPU — host stays responsive; limits documented.
6. Info leaks: /proc, /sys, mounts (C11), env, hostname, cgroup paths, kernel version → exploit hint.
7. Prompt injection via tool output (file in repo instructs agent to exfil) — run with real agent in F04.
DONE: every F07 sandbox-layer row has a case; all blocked or logged as GAP in caveats.md with mitigation.
GOOD: each case ≤3 lines in probe; creative cases have 1-line rationale.
Proof: `tests/attacks/run.sh` → table, exit 0 iff no unexpected FAIL.
Log:
