#!/bin/sh
# PoC F11: dev image works in a sandbox without the PATH workaround (C09)
# and has no setuid binaries (C12).
# Needs: scripts/build-dev-image.sh
# Usage: pocs/11-dev-image/run.sh [name]
set -u
cd "$(dirname "$0")/../.."

NAME="${1:-poc11}"
rc=0

trap 'openshell sandbox delete "$NAME" >/dev/null 2>&1' EXIT

run() { openshell sandbox exec -n "$NAME" -- sh -c "$1" 2>&1; }
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $2"; rc=1; }

scripts/sandbox-up.sh "$NAME" tests/fixtures/hello-go >/dev/null 2>&1 || { echo "FAIL create"; exit 1; }

out=$(run 'cd /sandbox/hello-go && go test ./...')
if echo "$out" | grep -q '^ok'; then pass "go test without PATH env"; else fail "go test without PATH env" "$out"; fi

out=$(run 'opencode --version')
if [ "$out" = 1.18.21 ]; then pass "opencode on PATH"; else fail "opencode on PATH" "$out"; fi

out=$(run 'git --version && rg --version | head -1')
if echo "$out" | grep -q ripgrep; then pass "git and rg"; else fail "git and rg" "$out"; fi

out=$(run 'find / -xdev -perm /6000 -type f 2>/dev/null | wc -l')
if [ "$out" = 0 ]; then pass "no setuid binaries"; else fail "no setuid binaries" "$out"; fi

out=$(run 'id -u')
if [ "$out" != 0 ]; then pass "non-root"; else fail "non-root" "$out"; fi

exit $rc
