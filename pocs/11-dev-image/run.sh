#!/bin/sh
# PoC F11: dev image works in a sandbox without PATH workaround (C09) and has no setuid bins (C12).
# Needs: scripts/build-dev-image.sh done. Usage: pocs/11-dev-image/run.sh [name]
set -u
NAME="${1:-poc11}"; ROOT="$(cd "$(dirname "$0")/../.." && pwd)"; rc=0
trap 'openshell sandbox delete "$NAME" >/dev/null 2>&1' EXIT
"$ROOT/scripts/sandbox-up.sh" "$NAME" "$ROOT/tests/fixtures/hello-go" >/dev/null 2>&1 || { echo "FAIL create"; exit 1; }
ex() { openshell sandbox exec -n "$NAME" -- sh -c "$1" 2>&1; }
chk() { if [ "$2" = 0 ]; then echo "PASS $1"; else echo "FAIL $1: $3"; rc=1; fi; }
o="$(ex 'cd /sandbox/hello-go && go test ./...')"; echo "$o" | grep -q '^ok' ; chk go-test-no-path-env $? "$o"
o="$(ex 'opencode --version')"; [ "$o" = 1.18.21 ]; chk opencode-on-path $? "$o"
o="$(ex 'git --version && rg --version | head -1')"; echo "$o" | grep -q ripgrep; chk git-rg $? "$o"
o="$(ex 'find / -xdev -perm /6000 -type f 2>/dev/null | wc -l')"; [ "$o" = 0 ]; chk no-setuid $? "$o"
o="$(ex 'id -u')"; [ "$o" != 0 ]; chk non-root $? "$o"
exit $rc
