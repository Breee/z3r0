#!/bin/sh
# PoC F01: a rootless OpenShell sandbox runs `go test` on an uploaded repo.
# Usage: pocs/01-openshell-baseline/run.sh [sandbox-name]
set -eu
cd "$(dirname "$0")/../.."

NAME="${1:-poc01}"
GO_PATH="PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"   # caveat C09

trap 'scripts/sandbox-down.sh "$NAME" >/dev/null 2>&1 || true' EXIT

echo "== Preflight"
podman --version
openshell status

echo "== Create sandbox"
scripts/sandbox-up.sh "$NAME" tests/fixtures/hello-go

echo "== Checks"
uid=$(openshell sandbox exec -n "$NAME" -- id -u)
if [ "$uid" = 0 ]; then
  echo "FAIL runs as root"
  exit 1
fi
echo "PASS non-root (uid=$uid)"

openshell sandbox exec -n "$NAME" --env "$GO_PATH" -- sh -c 'cd /sandbox/hello-go && go test ./...'
openshell sandbox exec -n "$NAME" -- git --version

echo "PASS"
