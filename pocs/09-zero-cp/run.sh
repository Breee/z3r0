#!/usr/bin/env bash
# PoC 09 (#90): zero-cp grants a token only to a registered sandbox identity.
# Needs the SPIRE + gateway environment of PoC 08 running (see README.md).
# Usage: OPENSHELL_SRC=<OpenShell v0.1.8 clone> [IMAGE=<image with curl>] run.sh
set -u
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
SPIRE=${OPENSHELL_SRC:?set OPENSHELL_SRC}/examples/spiffe-token-exchange-demo/podman/spire
IMAGE=${IMAGE:-$(sed -n 's/^const devImage = "\(.*\)"/\1/p' "$ROOT/internal/zero/zero.go")}
O="openshell --gateway spiffe-poc --gateway-endpoint http://127.0.0.1:18082"
URL=http://host.openshell.internal:18090
W=$(mktemp -d)
FAILED=0

pass() { echo "PASS $1"; }
fail() { echo "FAIL $1"; FAILED=1; }
check() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1 (got $2, want $3)"; fi; }
status() { $O sandbox exec --name "$1" -- curl -s -m 30 -o /dev/null -w '%{http_code}' "${@:2}"; }

cleanup() {
    kill "$CP" 2>/dev/null
    $O sandbox delete cp-a cp-b >/dev/null 2>&1
    $O provider delete zero-cp-poc >/dev/null 2>&1
    rm -rf "$W"
}
trap cleanup EXIT

go build -o "$W/zero-cp" "$ROOT/cmd/zero-cp" || exit 1
podman exec openshell-spiffe-demo-spire-server /opt/spire/bin/spire-server bundle show \
    -format spiffe -socketPath /run/spire/server/private/api.sock > "$W/jwks.json" || exit 1
: > "$W/allow.txt"
"$W/zero-cp" -jwks "$W/jwks.json" -audience zero-cp -allow "$W/allow.txt" -log "$W/decisions.jsonl" &
CP=$!

$O profile import --file "$ROOT/pocs/09-zero-cp/profile.yaml" >/dev/null 2>&1
$O provider create --name zero-cp-poc --type zero-cp-poc --runtime-credentials >/dev/null || exit 1
for n in a b; do
    $O sandbox create --name cp-$n --from "$IMAGE" --provider zero-cp-poc --detach >/dev/null || exit 1
done
sleep 10
for n in a b; do
    id=$($O sandbox get cp-$n --output json | jq -r .id)
    bash "$SPIRE/register-sandbox.sh" "$id" >/dev/null || exit 1
    eval "ID_$n=spiffe://openshell.local/openshell/sandbox/$id"
done
echo "$ID_a" > "$W/allow.txt"   # only sandbox a is registered in zero-cp
sleep 5

check "registered sandbox: request arrives with granted token" "$(status cp-a $URL/protected)" 200
check "unregistered sandbox (valid SVID): fails closed" "$(status cp-b $URL/protected)" 502
check "agent cannot call the token endpoint itself" "$(status cp-a -X POST -d client_assertion=x $URL/token)" 403
SEEN=$($O sandbox exec --name cp-a -- sh -c \
    'env; curl -s -m 30 -D- '$URL'/protected; grep -rs zerocp-test /tmp /sandbox /proc/[0-9]*/environ' | grep -c zerocp-test)
check "agent never sees the token (env, files, response)" "$SEEN" 0
FORGED=$(printf '{"alg":"ES256","kid":"x"}' | basenc --base64url -w0 | tr -d =).$(printf '{"sub":"%s","aud":["zero-cp"],"exp":9999999999}' "$ID_a" | basenc --base64url -w0 | tr -d =).AAAA
check "forged assertion denied" "$(curl -s -o /dev/null -w '%{http_code}' -d client_assertion="$FORGED" 127.0.0.1:18090/token)" 401
check "missing assertion denied" "$(curl -s -o /dev/null -w '%{http_code}' -X POST 127.0.0.1:18090/token)" 401
: > "$W/allow.txt"
check "registration removed: next request fails closed" "$(status cp-a $URL/protected)" 502

for want in "allow.*$ID_a" "deny.*sandbox not registered.*$ID_b" "deny.*unknown signing key" "deny.*missing client_assertion" "deny.*sandbox not registered.*$ID_a"; do
    if grep -q "$want" "$W/decisions.jsonl"; then pass "decision log: $want"; else fail "decision log: $want"; fi
done
echo "--- decision log"
cat "$W/decisions.jsonl"
exit $FAILED
