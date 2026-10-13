#!/bin/sh
# PoC 03: OpenShell credential placeholders.
# The agent only sees a placeholder. The proxy injects the real secret,
# and only for the bound endpoint (httpbin.org/anything). Needs internet.
# Usage: pocs/03-credential-placeholder/run.sh
set -u
cd "$(dirname "$0")"

NAME=cred03
SECRET="REAL-SECRET-$(date +%s)"
rc=0

cleanup() {
  openshell sandbox delete "$NAME" >/dev/null 2>&1
  openshell provider delete echo03 >/dev/null 2>&1
  sleep 5
  openshell profile delete z3r0-echo >/dev/null 2>&1
}
trap cleanup EXIT

run() { timeout 30 openshell sandbox exec -n "$NAME" -- sh -c "$1" 2>&1; }
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $2"; rc=1; }

echo "== Setup"
openshell profile lint -f profile.yaml >/dev/null
openshell profile import -f profile.yaml >/dev/null
openshell provider create --name echo03 --type z3r0-echo --credential "ECHO_TOKEN=$SECRET" >/dev/null
openshell sandbox create --name "$NAME" --from docker.io/library/golang:1.25 --provider echo03 --detach >/dev/null 2>&1

echo "== Checks"

out=$(run 'printf %s "$ECHO_TOKEN"')
case "$out" in
  openshell:resolve:env:*) pass "env holds placeholder" ;;
  *)                       fail "env holds placeholder" "$out" ;;
esac

out=$(run "grep -rsl '$SECRET' /proc/self/environ /sandbox /etc /tmp")
if [ -z "$out" ]; then pass "secret not in sandbox"
else fail "secret not in sandbox" "$out"; fi

out=$(run 'curl -s https://httpbin.org/anything -H "Authorization: Bearer $ECHO_TOKEN"')
if echo "$out" | grep -q "Bearer $SECRET"; then pass "upstream gets real secret"
else fail "upstream gets real secret" "$out"; fi

out=$(run 'curl -s -o /dev/null -w %{http_code} https://httpbin.org/headers -H "Authorization: Bearer $ECHO_TOKEN"')
if [ "$out" = 403 ]; then pass "unbound path denied"
else fail "unbound path denied" "$out"; fi

out=$(run 'curl -s https://httpbin.org/anything -H "Authorization: Bearer openshell:resolve:env:v1999_ECHO_TOKEN"')
if echo "$out" | grep -q credential_unavailable; then pass "forged placeholder denied"
else fail "forged placeholder denied" "$out"; fi

# agent sends its own (attacker) key to the allowed endpoint
out=$(run 'curl -s https://httpbin.org/anything -H "Authorization: Bearer ATTACKER-KEY"')
if echo "$out" | grep -q ATTACKER-KEY; then fail "foreign auth header blocked" "reached upstream"
else pass "foreign auth header blocked"; fi

out=$(run 'curl -s "https://httpbin.org/anything?api_key=ATTACKER-KEY" -H "X-Api-Key: ATTACKER-KEY"')
if echo "$out" | grep -q ATTACKER-KEY; then fail "foreign key in other fields blocked" "reached upstream"
else pass "foreign key in other fields blocked"; fi

exit $rc
