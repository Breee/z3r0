#!/usr/bin/env bash
# PoC 14 (#128): one agentgateway for all sessions, Kubernetes MCP server behind it,
# cluster credential (session ServiceAccount token) chosen per request by zero-cp.
# Needs the SPIRE + gateway environment of PoC 08 and the kind cluster of tests/k8s (see README.md).
# Usage: OPENSHELL_SRC=<OpenShell v0.1.8 clone> AGENTGATEWAY=<agentgateway v1.7.0> K8SMCP=<kubernetes-mcp-server v0.1.3>
#        ADMIN_KUBECONFIG=<admin> HUMAN_KUBECONFIG=<member of zero-users> AGENT_KUBECONFIG=<no credentials>
#        KIND_NODE=<node container> [IMAGE=<image with curl>] run.sh
set -u
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/pocs/14-k8s-mcp
SPIRE=${OPENSHELL_SRC:?set OPENSHELL_SRC}/examples/spiffe-token-exchange-demo/podman/spire
AG=${AGENTGATEWAY:?set AGENTGATEWAY}
KM=${K8SMCP:?set K8SMCP}
ADMIN=${ADMIN_KUBECONFIG:?set ADMIN_KUBECONFIG}
HUMAN=${HUMAN_KUBECONFIG:?set HUMAN_KUBECONFIG}
AGENTKC=${AGENT_KUBECONFIG:?set AGENT_KUBECONFIG}
NODE=${KIND_NODE:-zero-k8s-control-plane}
IMAGE=${IMAGE:-$(sed -n 's/^const devImage = "\(.*\)"/\1/p' "$ROOT/internal/zero/zero.go")}
O="openshell --gateway spiffe-poc --gateway-endpoint http://127.0.0.1:18082"
URL=http://host.openshell.internal:18091/mcp
W=$(mktemp -d)
FAILED=0
PIDS=

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
call() { mcp "$1" tools/call "{\"name\":\"$2\",\"arguments\":$3}"; }
session() { KUBECONFIG=$1 sh "$ROOT/tests/k8s/session.sh" "${@:2}"; }

cleanup() {
    kill $PIDS 2>/dev/null
    $O sandbox delete k8s-a k8s-b >/dev/null 2>&1
    sleep 2; $O provider delete zero-mcp-poc >/dev/null 2>&1
    session "$HUMAN" stop team-a p14a >/dev/null 2>&1
    session "$ADMIN" stop team-b p14b >/dev/null 2>&1
    rm -rf "$W"
}
trap cleanup EXIT

go build -o "$W/zero-cp" "$ROOT/pocs/09-zero-cp/zero-cp" || exit 1
podman exec openshell-spiffe-demo-spire-server /opt/spire/bin/spire-server bundle show \
    -format spiffe -socketPath /run/spire/server/private/api.sock > "$W/bundle.json" || exit 1
jq '{keys: [.keys[] | select(.use == "jwt-svid") | .use = "sig"]}' "$W/bundle.json" > "$W/jwks.json"
: > "$W/allow.txt"; mkdir "$W/sessions"
(umask 077; head -c 32 /dev/urandom | basenc --base64url -w0 > "$W/gateway.key")
"$W/zero-cp" -svid -jwks "$W/bundle.json" -audience zero-mcp -allow "$W/allow.txt" -sessions "$W/sessions" -gateway-key "$W/gateway.key" -log "$W/decisions.jsonl" &
PIDS=$!
sed "s#KUBECONFIG#$AGENTKC#" "$HERE/k8smcp.toml" > "$W/k8smcp.toml"
"$KM" --config "$W/k8smcp.toml" > "$W/k8smcp.log" 2>&1 &
PIDS="$PIDS $!"

$O profile import --file "$ROOT/pocs/13-agentgateway/profile.yaml" >/dev/null 2>&1
$O provider create --name zero-mcp-poc --type zero-mcp-poc --runtime-credentials >/dev/null || exit 1
for n in a b; do
    $O sandbox create --name k8s-$n --from "$IMAGE" --provider zero-mcp-poc --detach >/dev/null || exit 1
done
sleep 10
for n in a b; do
    id=$($O sandbox get k8s-$n --output json | jq -r .id)
    bash "$SPIRE/register-sandbox.sh" "$id" >/dev/null || exit 1
    eval "SB_$n=$id"
    echo "spiffe://openshell.local/openshell/sandbox/$id" >> "$W/allow.txt"
done
# what zero-cp would do on `zero up`: a ServiceAccount per session, bound to the Roles the policy lists
session "$HUMAN" start team-a p14a k8s.resource.read k8s.pod.logs > "$W/sessions/$SB_a" || exit 1
session "$ADMIN" start team-b p14b k8s.resource.read > "$W/sessions/$SB_b" || exit 1
TOKA=$(cat "$W/sessions/$SB_a")

sed -e "s#ISSUER#http://spire-oidc:8080#" -e "s#JWKS#$W/jwks.json#" -e "s#GWKEY#$W/gateway.key#" "$HERE/gateway.yaml" > "$W/gateway.yaml"
(cd "$W" && exec "$AG" -f gateway.yaml > "$W/gateway.log" 2>&1) &
PIDS="$PIDS $!"
sleep 8
POD=$(kubectl --kubeconfig "$ADMIN" -n team-a get pods -o jsonpath='{.items[0].metadata.name}')

LIST=$(mcp k8s-a tools/list '{}')
has "a: tools/list shows read tools" "$LIST" '"name":"pods_log"'
hasnot "a: tools outside the gateway rule are hidden (namespaces_list)" "$LIST" '"name":"namespaces_list"'
hasnot "a: no write tool offered (pods_delete)" "$LIST" '"name":"pods_delete"'
has "a: list pods in team-a" "$(call k8s-a pods_list_in_namespace '{"namespace":"team-a"}')" "$POD"
LOG=$(call k8s-a pods_log "{\"namespace\":\"team-a\",\"name\":\"$POD\"}")
hasnot "a: pod log works" "$LOG" '"isError":true'
has "a: pod log has content" "$LOG" '"text":"..'
has "a: Secrets denied by the cluster" "$(call k8s-a resources_list '{"apiVersion":"v1","kind":"Secret","namespace":"team-a"}')" 'forbidden.*zero-p14a'
has "a: other namespace denied by the cluster" "$(call k8s-a pods_list_in_namespace '{"namespace":"team-b"}')" 'forbidden.*zero-p14a'
has "a: write tool call denied" "$(call k8s-a pods_delete "{\"namespace\":\"team-a\",\"name\":\"$POD\"}")" 'error'
has "a: hidden tool call denied by the gateway" "$(call k8s-a namespaces_list '{}')" 'error'
has "b: same gateway, own ServiceAccount: list pods in team-b" "$(call k8s-b pods_list_in_namespace '{"namespace":"team-b"}')" 'team-b .*Running'
has "b: team-a denied, cluster names b's ServiceAccount" "$(call k8s-b pods_list_in_namespace '{"namespace":"team-a"}')" 'forbidden.*team-b:zero-p14b'
has "b: pod log denied (capability not granted)" "$(call k8s-b pods_log '{"namespace":"team-b","name":"x"}')" 'forbidden\|isError'
check "no identity (host, not through the supervisor): 401" \
    "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'content-type: application/json' -d '{}' 127.0.0.1:18091/mcp)" 401
check "cluster token as identity at the gateway: 401" \
    "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H "authorization: Bearer $TOKA" -H 'content-type: application/json' -d '{}' 127.0.0.1:18091/mcp)" 401
check "subject header without the gateway key at zero-cp: 403" \
    "$(curl -s -o /dev/null -w '%{http_code}' -H "x-zero-subject: spiffe://openshell.local/openshell/sandbox/$SB_a" 127.0.0.1:18090/authz/)" 403
SEEN=$($O sandbox exec --name k8s-a -- sh -c \
    'env; curl -s -m 30 -D- -X POST -H "content-type: application/json" -d "{}" '$URL'; grep -rs "eyJ[A-Za-z0-9_-]*\.eyJ" /tmp /sandbox /proc/[0-9]*/environ' | grep -c 'eyJ[A-Za-z0-9_-]*\.eyJ')
check "a: no token in the sandbox (env, files, response)" "$SEEN" 0
hasnot "a: tool results carry no token" "$LIST$LOG" 'eyJ[A-Za-z0-9_-]*\.eyJ'

docker exec "$NODE" cat /var/log/kubernetes/audit.log > "$W/audit.json"
AUD() { jq -r "select(.user.username == \"system:serviceaccount:$1\" and .objectRef.resource == \"$2\" and .objectRef.namespace == \"$3\") | \"\(.verb) \(.responseStatus.code)\"" "$W/audit.json" | sort | uniq -c | tr -s ' ' | tr '\n' ';'; }
has "audit: a's ServiceAccount listed pods in team-a" "$(AUD team-a:zero-p14a pods team-a)" 'list 200'
has "audit: a's ServiceAccount refused in team-b" "$(AUD team-a:zero-p14a pods team-b)" 'list 403'
has "audit: b's ServiceAccount listed pods in team-b" "$(AUD team-b:zero-p14b pods team-b)" 'list 200'

session "$HUMAN" stop team-a p14a >/dev/null
sleep 3
STOP=$(call k8s-a pods_list_in_namespace '{"namespace":"team-a"}')
hasnot "a: session stopped (ServiceAccount deleted): no pods" "$STOP" "$POD"
has "a: session stopped: cluster rejects the token" "$STOP" 'isError'
rm "$W/sessions/$SB_a"
has "a: session removed in zero-cp: gateway fails closed" "$(mcp k8s-a tools/list '{}')" 'no session'
has "b: still works after a's session ended" "$(call k8s-b pods_list_in_namespace '{"namespace":"team-b"}')" 'team-b .*Running'

[ -n "${KEEP:-}" ] && cp -r "$W" "$KEEP"
echo "--- gateway log (tool calls)"
grep -E 'gen_ai.tool.name' "$W/gateway.log" | sed -E 's/.*(http.status=[0-9]+).*(gen_ai.tool.name=[a-z_]+).*(jwt.sub=[^ \t]+).*/\1 \2 \3/' | cut -c1-300 | tail -n 8
echo "--- zero-cp decisions"
tail -n 4 "$W/decisions.jsonl" | cut -c1-300
exit $FAILED
