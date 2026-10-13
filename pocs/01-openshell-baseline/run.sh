#!/bin/sh
# PoC F01: a rootless OpenShell sandbox runs `go test` on an uploaded repo.
#
# Usage: pocs/01-openshell-baseline/run.sh [sandbox-name]
set -u
. "$(dirname "$0")/../lib.sh"

SANDBOX="${1:-poc01}"
SANDBOX_ENV="$GO_PATH"
trap '"$ROOT/scripts/sandbox-down.sh" "$SANDBOX" >/dev/null 2>&1' EXIT

step "Preflight"
podman --version
openshell status

step "Create sandbox and upload repo"
"$ROOT/scripts/sandbox-up.sh" "$SANDBOX" "$ROOT/tests/fixtures/hello-go" || exit 1

step "Checks"
uid="$(in_sandbox 'id -u')"
expect "runs-as-non-root (uid=$uid)" [ "$uid" != 0 ]
expect "go-test-passes"              in_sandbox 'cd /sandbox/hello-go && go test ./...'
expect "git-available"               in_sandbox 'git --version'

exit $RC
