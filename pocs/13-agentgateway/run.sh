#!/usr/bin/env bash
# PoC 13 (#127): agentgateway serves MCP tools only to a registered sandbox identity.
# Needs the SPIRE + gateway environment of PoC 08 running (see README.md).
# Usage: OPENSHELL_SRC=<OpenShell v0.1.8 clone> AGENTGATEWAY=<agentgateway v1.7.0 binary> [IMAGE=<image with curl>] run.sh
set -u
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/pocs/13-agentgateway
SPIRE=${OPENSHELL_SRC:?set OPENSHELL_SRC}/examples/spiffe-token-exchange-demo/podman/spire
AG=${AGENTGATEWAY:?set AGENTGATEWAY}
IMAGE=${IMAGE:-$(sed -n 's/^const devImage = "\(.*\)"/\1/p' "$ROOT/internal/zero/zero.go")}
O="openshell --gateway spiffe-poc --gateway-endpoint http://127.0.0.1:18082"
URL=http://host.openshell.internal:18091/mcp
W=$(mktemp -d)
FAILED=0
CP=
GW=

pass() { echo "PASS $1"; }
fail() { echo "FAIL $1"; FAILED=1; }
check() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1 (got $2, want $3)"; fi; }
has() { if printf '%s' "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1 (no '$3' in: $(printf '%s' "$2" | head -c 300))"; fi; }
hasnot() { if printf '%s' "$2" | grep -q -- "$3"; then fail "$1 (found '$3')"; else pass "$1"; fi; }

# mcp SANDBOX METHOD PARAMS: initialize a session, then send one request; prints the response.
mcp() {
    $O sandbox exec --name "$1" -- sh -c '
h1="content-type: application/json"; h2="accept: application/json, text/event-stream"
sid=$(curl -s -m 30 -D- -o /dev/null -H "$h1" -H "$h2" -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-06-18\",\"capabilities\":{},\"clientInfo\":{\"name\":\"poc\",\"version\":\"0\"}}}" '$URL' | sed -n "s/^mcp-session-id: //Ip" | tr -d "\r")
[ -n "$sid" ] || { echo "no session"; exit 0; }
curl -s -m 30 -o /dev/null -H "$h1" -H "$h2" -H "mcp-session-id: $sid" -d "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}" '$URL'
curl -s -m 60 -H "$h1" -H "$h2" -H "mcp-session-id: $sid" -d "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"$1\",\"params\":$2}" '$URL'
' sh "$2" "$3"
}
status() { $O sandbox exec --name "$1" -- curl -s -m 30 -o /dev/null -w '%{http_code}' -X POST -H 'content-type: application/json' -d '{}' "${@:2}"; }

cleanup() {
    kill $CP $GW 2>/dev/null
    $O sandbox delete mcp-a mcp-b >/dev/null 2>&1
    $O provider delete zero-mcp-poc >/dev/null 2>&1
    rm -rf "$W"
}
trap cleanup EXIT

go build -o "$W/zero-cp" "$ROOT/pocs/09-zero-cp/zero-cp" || exit 1
podman exec openshell-spiffe-demo-spire-server /opt/spire/bin/spire-server bundle show \
    -format spiffe -socketPath /run/spire/server/private/api.sock > "$W/bundle.json" || exit 1
# agentgateway wants a plain JWKS: only the JWT keys, each with a kid
jq '{keys: [.keys[] | select(.use == "jwt-svid") | .use = "sig"]}' "$W/bundle.json" > "$W/jwks.json"
: > "$W/allow.txt"
"$W/zero-cp" -svid -jwks "$W/bundle.json" -audience zero-mcp -allow "$W/allow.txt" -log "$W/decisions.jsonl" &
CP=$!

$O profile import --file "$HERE/profile.yaml" >/dev/null 2>&1
$O provider create --name zero-mcp-poc --type zero-mcp-poc --runtime-credentials >/dev/null || exit 1
for n in a b; do
    $O sandbox create --name mcp-$n --from "$IMAGE" --provider zero-mcp-poc --detach >/dev/null || exit 1
done
sleep 10
for n in a b; do
    id=$($O sandbox get mcp-$n --output json | jq -r .id)
    bash "$SPIRE/register-sandbox.sh" "$id" >/dev/null || exit 1
    eval "ID_$n=spiffe://openshell.local/openshell/sandbox/$id"
done
printf '%s\n%s\n' "$ID_a" "$ID_b" > "$W/allow.txt"   # zero-cp lets both through; agentgateway allows tools for a only

sed -e "s#ISSUER#http://spire-oidc:8080#" -e "s#JWKS#$W/jwks.json#" -e "s#ALLOWED_ID#$ID_a#" "$HERE/gateway.yaml" > "$W/gateway.yaml"
(cd "$W" && exec "$AG" -f gateway.yaml > "$W/gateway.log" 2>&1) &
GW=$!
sleep 8

LIST=$(mcp mcp-a tools/list '{}')
has "sandbox a: tools/list shows the allowed tool" "$LIST" '"name":"echo"'
hasnot "sandbox a: tools/list hides every other tool" "$LIST" '"name":"get-sum"'
has "sandbox a: allowed tool call works" "$(mcp mcp-a tools/call '{"name":"echo","arguments":{"message":"hello-13"}}')" 'hello-13'
CALL=$(mcp mcp-a tools/call '{"name":"get-sum","arguments":{"a":1,"b":2}}')
hasnot "sandbox a: tool outside the rule is denied" "$CALL" '"text":"The sum'
has "sandbox a: denial is an error" "$CALL" 'error'
LISTB=$(mcp mcp-b tools/list '{}')
hasnot "sandbox b (valid identity, no rule): sees no tool" "$LISTB" '"name":"echo"'
hasnot "sandbox b: tool call denied" "$(mcp mcp-b tools/call '{"name":"echo","arguments":{"message":"hello-b"}}')" '"text":"Echo'
check "no token (host, not through the supervisor): 401" \
    "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'content-type: application/json' -d '{}' 127.0.0.1:18091/mcp)" 401
FORGED=$(printf '{"alg":"ES256","kid":"x"}' | basenc --base64url -w0 | tr -d =).$(printf '{"sub":"%s","iss":"http://spire-oidc:8080","aud":["zero-mcp"],"exp":9999999999}' "$ID_a" | basenc --base64url -w0 | tr -d =).AAAA
check "forged token: 401" \
    "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H "authorization: Bearer $FORGED" -H 'content-type: application/json' -d '{}' 127.0.0.1:18091/mcp)" 401
# 406 = authenticated, then refused for the missing Accept header; the forged header alone gives 401 (check above)
check "agent's own Authorization header is replaced by the supervisor" \
    "$(status mcp-b -H "authorization: Bearer $FORGED" $URL)" 406
SEEN=$($O sandbox exec --name mcp-a -- sh -c \
    'env; curl -s -m 30 -D- -X POST -H "content-type: application/json" -d "{}" '$URL'; grep -rs "eyJ[A-Za-z0-9_-]*\.eyJ" /tmp /sandbox /proc/[0-9]*/environ' | grep -c 'eyJ[A-Za-z0-9_-]*\.eyJ')
check "agent never sees the token (env, files, response)" "$SEEN" 0
: > "$W/allow.txt"
sleep 1
has "registration removed in zero-cp: next request fails closed" "$(mcp mcp-a tools/list '{}')" 'no session'

if grep -q "jwt.sub=$ID_a" "$W/gateway.log" || grep -q "$ID_a" "$W/gateway.log"; then pass "gateway log names the sandbox identity"; else fail "gateway log names the sandbox identity"; fi
echo "--- gateway log (tool calls)"
grep -E 'mcp\.|401' "$W/gateway.log" | cut -c1-400 | tail -n 12
echo "--- zero-cp decisions"
tail -n 6 "$W/decisions.jsonl"
exit $FAILED
