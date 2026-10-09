#!/bin/sh
# PoC 03: OpenShell credential placeholders. Agent sees placeholder; proxy injects real secret
# only for the bound endpoint (httpbin.org/anything). Needs internet.
# Usage: pocs/03-credential-placeholder/run.sh
set -u
cd "$(dirname "$0")"
SECRET="REAL-SECRET-$(date +%s)"; N=cred03; rc=0
cleanup() {
  openshell sandbox delete "$N" >/dev/null 2>&1
  openshell provider delete echo03 >/dev/null 2>&1
  sleep 5; openshell profile delete z3r0-echo >/dev/null 2>&1
}
trap cleanup EXIT
X() { timeout 30 openshell sandbox exec -n "$N" -- sh -c "$1" 2>&1; }
check() { if [ "$2" = ok ]; then echo "PASS $1"; else echo "FAIL $1 ($3)"; rc=1; fi; }

openshell profile lint -f profile.yaml >/dev/null && openshell profile import -f profile.yaml >/dev/null
openshell provider create --name echo03 --type z3r0-echo --credential "ECHO_TOKEN=$SECRET" >/dev/null
openshell sandbox create --name "$N" --from docker.io/library/golang:1.25 --provider echo03 --detach >/dev/null 2>&1

v=$(X 'printf %s "$ECHO_TOKEN"')
case "$v" in openshell:resolve:env:*) check env-is-placeholder ok;; *) check env-is-placeholder no "$v";; esac
o=$(X "grep -rsl '$SECRET' /proc/self/environ /sandbox /etc /tmp")
[ -z "$o" ] && check secret-not-in-sandbox ok || check secret-not-in-sandbox no "$o"
o=$(X 'curl -s https://httpbin.org/anything -H "Authorization: Bearer $ECHO_TOKEN"')
echo "$o" | grep -q "Bearer $SECRET" && check upstream-gets-real-secret ok || check upstream-gets-real-secret no "$o"
o=$(X 'curl -s -o /dev/null -w %{http_code} https://httpbin.org/headers -H "Authorization: Bearer $ECHO_TOKEN"')
[ "$o" = 403 ] && check unbound-path-denied ok || check unbound-path-denied no "$o"
o=$(X 'curl -s https://httpbin.org/anything -H "Authorization: Bearer openshell:resolve:env:v1999_ECHO_TOKEN"')
echo "$o" | grep -q credential_unavailable && check forged-placeholder-denied ok || check forged-placeholder-denied no "$o"
exit $rc
