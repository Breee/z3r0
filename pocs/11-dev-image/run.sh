#!/bin/sh
# PoC F11: the dev image works in a sandbox without the PATH workaround (C09)
# and ships no setuid binaries (C12).
#
# Needs: scripts/build-dev-image.sh
# Usage: pocs/11-dev-image/run.sh [sandbox-name]
set -u
. "$(dirname "$0")/../lib.sh"

SANDBOX="${1:-poc11}"
trap delete_sandbox EXIT

step "Setup"
"$ROOT/scripts/sandbox-up.sh" "$SANDBOX" "$ROOT/tests/fixtures/hello-go" >/dev/null 2>&1 \
  || { fail create; exit 1; }

step "Checks"
go_test="$(in_sandbox 'cd /sandbox/hello-go && go test ./...')"
oc_version="$(in_sandbox 'opencode --version')"
tools="$(in_sandbox 'git --version && rg --version | head -1')"
setuid_count="$(in_sandbox 'find / -xdev -perm /6000 -type f 2>/dev/null | wc -l')"
uid="$(in_sandbox 'id -u')"

expect go-test-no-path-env contains "$go_test" '^ok'
expect opencode-on-path    [ "$oc_version" = 1.18.21 ]
expect git-and-ripgrep     contains "$tools" ripgrep
expect no-setuid-binaries  [ "$setuid_count" = 0 ]
expect non-root            [ "$uid" != 0 ]

exit $RC
