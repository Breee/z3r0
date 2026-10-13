#!/bin/sh
# PoC 04: OpenCode runs inside an OpenShell sandbox (Option A).
# The agent edits, runs go test and commits, all inside the sandbox.
# Only the opencode binary may reach opencode.ai:443.
# Usage: pocs/04-opencode-in-sandbox/run.sh [name] [model]
set -u
cd "$(dirname "$0")/../.."

NAME="${1:-poc04}"
MODEL="${2:-opencode/nemotron-3.5-lightning-free}"
GO_PATH="PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"
TMP=$(mktemp -d)
rc=0

trap 'openshell sandbox delete "$NAME" >/dev/null 2>&1; rm -rf "$TMP"' EXIT

run() { openshell sandbox exec -n "$NAME" --env "$GO_PATH" -- sh -c "$1" 2>&1; }
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1"; rc=1; }

echo "== Setup"
scripts/sandbox-up.sh "$NAME" tests/fixtures/hello-go >/dev/null || exit 1

mkdir -p "$TMP/ocbin"
cp -L "$(command -v opencode)" "$TMP/ocbin/opencode"
openshell sandbox upload "$NAME" "$TMP/ocbin" /sandbox >/dev/null || exit 1
openshell policy update "$NAME" --rule-name zen --binary /sandbox/ocbin/opencode \
  --add-endpoint opencode.ai:443:read-write:rest:enforce --wait >/dev/null || exit 1

run 'cd /sandbox/hello-go && git init -q && git add -A && git -c user.name=a -c user.email=a@b commit -qm init' >/dev/null

echo "== Agent"
run "cd /sandbox/hello-go && timeout 300 /sandbox/ocbin/opencode run -m $MODEL \
  'Add func Bye() string returning \"bye\" to hello.go and a test in hello_test.go. Run go test ./... . Then run: git -c user.name=agent -c user.email=agent@z3r0 commit -am \"add Bye\". Reply DONE.'" \
  > "$TMP/agent.log"
tail -3 "$TMP/agent.log"

echo "== Checks"
if run 'cd /sandbox/hello-go && git log --format=%s -1' | grep -q '^add Bye'; then pass "agent committed"; else fail "agent committed"; fi
if run 'cd /sandbox/hello-go && grep -c "func Bye" hello.go' | grep -qx 1; then pass "agent edited"; else fail "agent edited"; fi
if run 'cd /sandbox/hello-go && go test ./...' | grep -q '^ok'; then pass "go test"; else fail "go test"; fi

# opencode.ai is only allowed for the opencode binary, not for curl
if run 'curl -s -m 5 -o /dev/null https://opencode.ai' >/dev/null; then fail "zen only for opencode binary"; else pass "zen only for opencode binary"; fi

if [ -e tests/fixtures/hello-go/.git ]; then fail "host untouched"; else pass "host untouched"; fi

exit $rc
