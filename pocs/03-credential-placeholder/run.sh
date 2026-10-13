#!/bin/sh
# PoC 03: OpenShell credential placeholders.
#
# The agent only ever sees a placeholder. The proxy swaps in the real secret,
# and only for the bound endpoint (httpbin.org/anything). Needs internet.
#
# Usage: pocs/03-credential-placeholder/run.sh
set -u
. "$(dirname "$0")/../lib.sh"
cd "$(dirname "$0")"

SANDBOX=cred03
PROVIDER=echo03
SECRET="REAL-SECRET-$(date +%s)"

cleanup() {
  delete_sandbox
  openshell provider delete "$PROVIDER" >/dev/null 2>&1
  sleep 5   # profile can't be deleted while the provider is still going away
  openshell profile delete z3r0-echo >/dev/null 2>&1
}
trap cleanup EXIT

# every sandbox call gets a 30s cap
sb() { timeout 30 openshell sandbox exec -n "$SANDBOX" -- sh -c "$1" 2>&1; }

# ---- setup --------------------------------------------------------------

step "Setup"
openshell profile lint   -f profile.yaml >/dev/null || exit 1
openshell profile import -f profile.yaml >/dev/null || exit 1
openshell provider create --name "$PROVIDER" --type z3r0-echo \
  --credential "ECHO_TOKEN=$SECRET" >/dev/null || exit 1
openshell sandbox create --name "$SANDBOX" --provider "$PROVIDER" \
  --from docker.io/library/golang:1.25 --detach >/dev/null 2>&1 || exit 1

# ---- checks -------------------------------------------------------------

step "Secret stays outside the sandbox"

token="$(sb 'printf %s "$ECHO_TOKEN"')"
case "$token" in
  openshell:resolve:env:*) pass env-is-placeholder ;;
  *)                       fail env-is-placeholder "$token" ;;
esac

found="$(sb "grep -rsl '$SECRET' /proc/self/environ /sandbox /etc /tmp")"
if [ -z "$found" ]; then pass secret-not-in-sandbox
else                     fail secret-not-in-sandbox "$found"
fi

step "Proxy injects the secret only where allowed"

out="$(sb 'curl -s https://httpbin.org/anything -H "Authorization: Bearer $ECHO_TOKEN"')"
if contains "$out" "Bearer $SECRET"; then pass upstream-gets-real-secret
else                                      fail upstream-gets-real-secret "$out"
fi

code="$(sb 'curl -s -o /dev/null -w %{http_code} https://httpbin.org/headers -H "Authorization: Bearer $ECHO_TOKEN"')"
if [ "$code" = 403 ]; then pass unbound-path-denied
else                       fail unbound-path-denied "HTTP $code"
fi

out="$(sb 'curl -s https://httpbin.org/anything -H "Authorization: Bearer openshell:resolve:env:v1999_ECHO_TOKEN"')"
if contains "$out" credential_unavailable; then pass forged-placeholder-denied
else                                            fail forged-placeholder-denied "$out"
fi

step "Agent can't smuggle its own key to the allowed endpoint"

out="$(sb 'curl -s https://httpbin.org/anything -H "Authorization: Bearer ATTACKER-KEY"')"
if contains "$out" ATTACKER-KEY; then fail foreign-auth-header-blocked "reached upstream"
else                                  pass foreign-auth-header-blocked
fi

out="$(sb 'curl -s "https://httpbin.org/anything?api_key=ATTACKER-KEY" -H "X-Api-Key: ATTACKER-KEY"')"
if contains "$out" ATTACKER-KEY; then fail foreign-key-other-fields-blocked "reached upstream"
else                                  pass foreign-key-other-fields-blocked
fi

exit $RC
