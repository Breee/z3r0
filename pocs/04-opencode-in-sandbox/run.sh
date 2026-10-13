#!/bin/sh
# PoC 04: OpenCode runs inside an OpenShell sandbox (Option A).
#
# The agent edits code, runs `go test` and commits, all inside the sandbox.
# Only the opencode binary may reach opencode.ai:443; nothing else gets out.
#
# Usage: pocs/04-opencode-in-sandbox/run.sh [sandbox-name] [model]
set -u
. "$(dirname "$0")/../lib.sh"

SANDBOX="${1:-poc04}"
MODEL="${2:-$DEFAULT_MODEL}"
SANDBOX_ENV="$GO_PATH"
TMP="$(mktemp -d)"
trap 'delete_sandbox; rm -rf "$TMP"' EXIT

TASK='Add func Bye() string returning "bye" to hello.go and a test in hello_test.go.
Run go test ./... .
Then run: git -c user.name=agent -c user.email=agent@z3r0 commit -am "add Bye".
Reply DONE.'

step "Setup"
"$ROOT/scripts/sandbox-up.sh" "$SANDBOX" "$ROOT/tests/fixtures/hello-go" >/dev/null || exit 1
upload_opencode "$TMP" || exit 1
git_init_in_sandbox /sandbox/hello-go

step "Agent works on the repo"
in_sandbox "cd /sandbox/hello-go && timeout 300 /sandbox/ocbin/opencode run -m $MODEL '$TASK'" > "$TMP/agent.log"
tail -3 "$TMP/agent.log"

step "Checks"
last_commit="$(in_sandbox 'cd /sandbox/hello-go && git log --format=%s -1')"
bye_count="$(in_sandbox 'cd /sandbox/hello-go && grep -c "func Bye" hello.go')"
test_out="$(in_sandbox 'cd /sandbox/hello-go && go test ./... 2>&1 | tail -1')"

expect agent-committed contains "$last_commit" '^add Bye'
expect agent-edited    [ "$bye_count" = 1 ]
expect go-test         contains "$test_out" '^ok'

# opencode.ai is allowed for the opencode binary only, not for curl
expect_not zen-only-for-opencode-binary \
  in_sandbox 'curl -s -m 5 -o /dev/null https://opencode.ai'

expect_not host-untouched test -e "$ROOT/tests/fixtures/hello-go/.git"

exit $RC
