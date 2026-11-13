#!/usr/bin/env bash
# PoC 10 (#88): OpenShell gateway stores provider credentials in OpenBao (vault driver).
# Starts a throwaway OpenBao dev server and a throwaway gateway as the calling user.
# Needs: bao, openshell, openshell-gateway, a rootless podman socket, openssl, jq, python3.
# Usage: [BAO=<bao binary>] [GATEWAY=<openshell-gateway binary>] [IMAGE=<image with curl>] run.sh
set -u
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
BAO=${BAO:-bao}
GATEWAY=${GATEWAY:-/usr/local/lib/zero/openshell-0.1.8/openshell-gateway}
IMAGE=${IMAGE:-$(sed -n 's/^const devImage = "\(.*\)"/\1/p' "$ROOT/internal/zero/zero.go")}
SOCK=${PODMAN_SOCKET:-$XDG_RUNTIME_DIR/podman/podman.sock}
O="openshell --gateway bao-poc --gateway-endpoint http://127.0.0.1:18092"
URL=http://host.openshell.internal:18090
W=$(mktemp -d)
MARK=poc10-secret-$$-$(openssl rand -hex 8)
FAILED=0
export BAO_ADDR=http://127.0.0.1:8200 BAO_TOKEN=root-poc

pass() { echo "PASS $1"; }
fail() { echo "FAIL $1"; FAILED=1; }
check() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1 (got $2, want $3)"; fi; }
status() { $O sandbox exec --name "$1" -- sh -c "curl -s -m 30 -o /dev/null -w '%{http_code}' -H \"Authorization: Bearer \$POC_TOKEN\" $URL/protected"; }

cleanup() {
    $O sandbox delete bao-a bao-b >/dev/null 2>&1
    $O provider delete bao-poc bao-x >/dev/null 2>&1
    kill "$CP" "$GW" "$BP" 2>/dev/null
    rm -rf "$W"
}
trap cleanup EXIT

# Protected test service: 200 only for the real secret as bearer token.
python3 -c '
import http.server, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200 if self.headers.get("Authorization") == "Bearer " + sys.argv[1] else 401)
        self.end_headers()
    def log_message(self, *a): pass
http.server.HTTPServer(("0.0.0.0", 18090), H).serve_forever()' "$MARK" &
CP=$!

# OpenBao dev server + token scoped to the KV mount.
"$BAO" server -dev -dev-root-token-id=root-poc -dev-listen-address=127.0.0.1:8200 >"$W/bao.log" 2>&1 &
BP=$!
sleep 2
echo 'path "secret/*" { capabilities = ["create","read","update","delete","list"] }' | "$BAO" policy write openshell - >/dev/null || exit 1
(umask 077; "$BAO" token create -policy=openshell -ttl=1h -format=json | jq -r .auth.client_token > "$W/bao-token")

# Throwaway gateway with the vault credential driver.
mkdir -p "$W/jwt"
openssl genpkey -algorithm ed25519 -out "$W/jwt/signing.pem" 2>/dev/null
openssl pkey -in "$W/jwt/signing.pem" -pubout -out "$W/jwt/public.pem"
openssl rand -hex 8 > "$W/jwt/kid"
cat > "$W/gateway.toml" <<EOF
[openshell]
version = 2

[openshell.gateway]
bind_address = "127.0.0.1:18092"
health_bind_address = "127.0.0.1:18093"
compute_driver = "podman"
disable_tls = true
credential_drivers = ["vault"]

[openshell.credential_drivers.vault]
address = "http://127.0.0.1:8200"
mount = "secret"
kv_version = "2"
auth_method = "token_file"
token_path = "$W/bao-token"

[openshell.gateway.auth]
allow_unauthenticated_users = true

[openshell.gateway.gateway_jwt]
signing_key_path = "$W/jwt/signing.pem"
public_key_path = "$W/jwt/public.pem"
kid_path = "$W/jwt/kid"
gateway_id = "bao-poc"
ttl_secs = 3600

[openshell.drivers.podman]
socket_path = "$SOCK"
network_name = "openshell"
grpc_endpoint = "http://127.0.0.1:18092"
image_pull_policy = "if_not_present"
stop_timeout_secs = 3
supervisor_image = "ghcr.io/nvidia/openshell/supervisor:0.1.8"
EOF
env -i PATH="$PATH" HOME="$W" XDG_STATE_HOME="$W/state" XDG_CONFIG_HOME="$W/config" XDG_DATA_HOME="$W/data" \
    XDG_RUNTIME_DIR="$XDG_RUNTIME_DIR" OPENSHELL_GATEWAY_NAME=bao-poc OPENSHELL_DB_URL="sqlite:$W/gateway.db?mode=rwc" \
    "$GATEWAY" --config "$W/gateway.toml" >"$W/gw.log" 2>&1 &
GW=$!
for _ in $(seq 30); do curl -sf -o /dev/null http://127.0.0.1:18093/healthz && break; sleep 1; done

$O profile import --file "$(dirname "$0")/profile.yaml" >/dev/null 2>&1
POC_TOKEN=$MARK $O provider create --name bao-poc --type bao-poc --credential POC_TOKEN >/dev/null || { cat "$W/gw.log"; exit 1; }
$O sandbox create --name bao-a --from "$IMAGE" --provider bao-poc --detach >/dev/null || exit 1
sleep 12

check "sandbox uses the credential (placeholder substituted)" "$(status bao-a)" 200
SEEN=$($O sandbox exec --name bao-a -- sh -c 'printf %s "$POC_TOKEN"')
case $SEEN in openshell:resolve:*) pass "agent sees only the placeholder";; *) fail "agent sees: $SEEN";; esac
check "secret absent from gateway state (db, files)" "$(grep -rl "$MARK" "$W/gateway.db"* "$W/state" "$W/data" 2>/dev/null | wc -l)" 0
HITS=0
for k in $("$BAO" kv list -format=json secret/openshell/provider-credentials | jq -r '.[]'); do
    "$BAO" kv get -format=json "secret/openshell/provider-credentials/$k" | grep -q "$MARK" && HITS=$((HITS + 1))
done
check "secret present in OpenBao KV" "$HITS" 1

kill "$BP"; sleep 2
POC_TOKEN=x $O provider create --name bao-x --type bao-poc --credential POC_TOKEN >/dev/null 2>&1
check "OpenBao down: new credential refused" "$?" 1
$O sandbox create --name bao-b --from "$IMAGE" --provider bao-poc --detach >/dev/null 2>&1
check "OpenBao down: new sandbox with the credential refused" "$?" 1
echo "INFO OpenBao down: running sandbox still gets $(status bao-a) (supervisor keeps the resolved secret)"

exit $FAILED
