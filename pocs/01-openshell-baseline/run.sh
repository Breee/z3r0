#!/bin/sh
# PoC F01: rootless OpenShell sandbox runs `go test` on an uploaded repo.
# Usage: pocs/01-openshell-baseline/run.sh [sandbox-name]
set -eu
cd "$(dirname "$0")/../.."
NAME="${1:-poc01}"
P='PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin'   # caveat C09
ex() { openshell sandbox exec -n "$NAME" --env "$P" -- "$@"; }

trap 'scripts/sandbox-down.sh "$NAME" >/dev/null 2>&1 || true' EXIT

echo "== preflight"
podman --version
openshell status

echo "== create + upload"
scripts/sandbox-up.sh "$NAME" tests/fixtures/hello-go

echo "== checks"
uid=$(ex id -u)
[ "$uid" != 0 ] || { echo "FAIL: runs as root"; exit 1; }
echo "uid=$uid (not root) OK"
ex sh -c 'cd /sandbox/hello-go && go test ./...'
ex git --version

echo "PASS"
