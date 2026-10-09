# F04 — Review

Commands to verify this feature. Run in order from repo root; expected result after each.

## Step 1: OpenCode inside sandbox (Option A)
```sh
pocs/04-opencode-in-sandbox/run.sh; echo rc=$?
```
Expected: `PASS agent-committed`, `agent-edited`, `go-test`, `zen-only-for-opencode-binary`, `host-untouched`; `rc=0` (~2 min; LLM output varies).
## Step 2: Host plugin (Option B)
```sh
cat pocs/04b-host-plugin/.opencode/plugins/sandbox-bash.ts
pocs/04b-host-plugin/run.sh; echo rc=$?
```
Expected: `PASS bash-routed-to-sandbox`, `GAP read-tool-runs-on-host` (agent reads host file `host-marker.txt`), `rc=0`.
## Step 3: Agent-driven proof + prompt injection (A07)
```sh
pocs/04-opencode-in-sandbox/attack.sh; echo rc=$?
grep -E 'curl|policy_denied|canary' /tmp/opencode/inject.log | head
```
Expected: `INFO probe lines: 45 PASS, 0 FAIL`, `PASS agent-probe-all-blocked`, `INFO agent-followed-injection`, `GAP inject:real-secret-in-agent-context (C27)`, PASS host-canary-unreadable / secret-not-in-env / post-exfil-denied / attacker-host-unreached; `rc=0` (~4 min).
