#!/bin/sh
# Attack suite (host side). Usage: tests/attacks/run.sh [sandbox-name]
set -eu
cd "$(dirname "$0")/../.."
NAME="${1:-f08}"
trap 'scripts/sandbox-down.sh "$NAME" >/dev/null 2>&1 || true' EXIT
scripts/sandbox-up.sh "$NAME" tests/attacks >/dev/null
rc=0
openshell sandbox exec -n "$NAME" -- sh /sandbox/attacks/egress.sh deny || rc=1
openshell policy update "$NAME" --rule-name httpbin --binary /usr/bin/curl \
  --add-endpoint httpbin.org:443:read-only:rest:enforce --wait >/dev/null
openshell sandbox exec -n "$NAME" -- sh /sandbox/attacks/egress.sh allow || rc=1
exit $rc
