#!/bin/sh
# PoC04: OpenCode runs inside an OpenShell sandbox (Option A).
# edit -> go test -> git commit, all inside the sandbox. Only opencode.ai:443 allowed, only for the opencode binary.
# Usage: pocs/04-opencode-in-sandbox/run.sh [name] [model]
set -u
NAME="${1:-poc04}"; MODEL="${2:-opencode/nemotron-3.5-lightning-free}"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"; rc=0
cleanup() { openshell sandbox delete "$NAME" >/dev/null 2>&1; rm -rf "$TMP"; }
trap cleanup EXIT
PATHENV="PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"
ok() { echo "PASS $1"; }; ko() { echo "FAIL $1"; rc=1; }

"$ROOT/scripts/sandbox-up.sh" "$NAME" "$ROOT/tests/fixtures/hello-go" >/dev/null || exit 1
mkdir -p "$TMP/ocbin" && cp -L "$(command -v opencode)" "$TMP/ocbin/opencode"
openshell sandbox upload "$NAME" "$TMP/ocbin" /sandbox >/dev/null || exit 1
openshell policy update "$NAME" --rule-name zen --binary /sandbox/ocbin/opencode \
  --add-endpoint opencode.ai:443:read-write:rest:enforce --wait >/dev/null || exit 1

openshell sandbox exec -n "$NAME" --env "$PATHENV" -- sh -c '
  cd /sandbox/hello-go && git init -q && git -c user.name=a -c user.email=a@b add -A && git -c user.name=a -c user.email=a@b commit -qm init' >/dev/null

openshell sandbox exec -n "$NAME" --env "$PATHENV" -- sh -c "cd /sandbox/hello-go && timeout 300 /sandbox/ocbin/opencode run -m $MODEL \
 'Add func Bye() string returning \"bye\" to hello.go and a test in hello_test.go. Run go test ./... . Then run: git -c user.name=agent -c user.email=agent@z3r0 commit -am \"add Bye\". Reply DONE.'" >"$TMP/agent.log" 2>&1
tail -3 "$TMP/agent.log"

out="$(openshell sandbox exec -n "$NAME" --env "$PATHENV" -- sh -c 'cd /sandbox/hello-go && git log --format=%s -1 && grep -c "func Bye" hello.go && go test ./... 2>&1 | tail -1')"
echo "$out" | grep -q '^add Bye' && ok agent-committed || ko agent-committed
echo "$out" | grep -qx 1 && ok agent-edited || ko agent-edited
echo "$out" | grep -q '^ok' && ok go-test || ko go-test
openshell sandbox exec -n "$NAME" -- sh -c 'curl -s -m 5 -o /dev/null https://opencode.ai' >/dev/null 2>&1 \
  && ko zen-only-for-opencode-binary || ok zen-only-for-opencode-binary
test -e "$ROOT/tests/fixtures/hello-go/.git" && ko host-untouched || ok host-untouched
exit $rc
