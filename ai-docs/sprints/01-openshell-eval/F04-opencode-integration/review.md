# F04 — Review

Commands to verify this feature. Run in order from repo root; expected result after each.

## Step 1: OpenCode inside sandbox (Option A)
```sh
pocs/04-opencode-in-sandbox/run.sh; echo rc=$?
```
Expected: `PASS agent-committed`, `agent-edited`, `go-test`, `zen-only-for-opencode-binary`, `host-untouched`; `rc=0` (~2 min; LLM output varies).
