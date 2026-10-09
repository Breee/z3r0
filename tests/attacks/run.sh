#!/bin/sh
# Attack suite (host side). PASS=blocked, GAP=known gap (caveats.md), FAIL=unexpected.
# Usage: tests/attacks/run.sh [sandbox-name]   (needs internet: httpbin.org, postman-echo.com)
set -eu
cd "$(dirname "$0")/../.."
NAME="${1:-f08}"; P=pocs/03-credential-placeholder/profile.yaml
SECRET="REAL-SECRET-$(date +%s)"
cleanup() {
  scripts/sandbox-down.sh "$NAME" >/dev/null 2>&1 || true
  openshell provider delete echo08 >/dev/null 2>&1 || true
  sleep 5; openshell profile delete z3r0-echo >/dev/null 2>&1 || true
}
trap cleanup EXIT
pol() { openshell policy update "$NAME" --rule-name "$1" --binary /usr/bin/curl --add-endpoint "$2" --wait >/dev/null 2>&1; }
run() { openshell sandbox exec -n "$NAME" -- sh "/sandbox/attacks/$1" "$2" || rc=1; }

openshell profile import -f "$P" >/dev/null
openshell provider create --name echo08 --type z3r0-echo --credential "ECHO_TOKEN=$SECRET" >/dev/null
openshell sandbox create --name "$NAME" --from docker.io/library/golang:1.25 --provider echo08 --detach >/dev/null 2>&1
openshell sandbox upload "$NAME" tests/attacks /sandbox >/dev/null 2>&1
rc=0
run egress.sh deny                         # step 1: provider endpoint only (/anything)
pol httpbin  httpbin.org:443:read-only:rest:enforce
run egress.sh allow
pol postman  postman-echo.com:443:read-only:rest:enforce
run exfil.sh "$SECRET"                     # step 2
run gateway.sh -                           # step 3
exit $rc
