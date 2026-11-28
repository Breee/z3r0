#!/bin/sh
# PoC 36 (#179): one sign-in for LiteLLM's page through its OpenID sign-in.
# Own compose project with zero's two images and a tiny provider; the
# running model proxy of zero is not touched. Secrets are made here and
# never printed.
set -eu
cd "$(dirname "$0")"

PORT=${PORT:-18796}
IDP=${IDP:-18797}
ROOT=${ROOT:-/litellm}
P=zero-poc36
BASE=http://127.0.0.1:$PORT
D=$(mktemp -d)
chmod 700 "$D"

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-200)"; fails=$((fails+1)); }
is() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "got $2, want $3"; fi; }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
dc() { docker --context default compose -p $P --env-file "$D/env" -f "$D/compose.yaml" "$@"; }

cleanup() {
	dc down -v >/dev/null 2>&1 || echo "CLEANUP FAILED: docker compose -p $P down -v"
	rm -rf "$D"
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

cat > "$D/env" <<ENV
MASTER=sk-$(openssl rand -hex 24)
DBPW=$(openssl rand -hex 16)
SECRET=$(openssl rand -hex 24)
ENV
chmod 600 "$D/env"
cp idp.py "$D/idp.py"
cat > "$D/config.yaml" <<CFG
general_settings:
  disable_password_login_when_sso_enabled: true
CFG
cat > "$D/compose.yaml" <<YAML
services:
  db:
    image: docker.io/library/postgres@sha256:f9cf3e1f0066f0eb0c48707d864f0ac5941e9ff454a072077d97a4d278d07d73
    environment: {POSTGRES_USER: litellm, POSTGRES_DB: litellm, POSTGRES_PASSWORD: "\${DBPW}"}
    healthcheck: {test: ["CMD-SHELL", "pg_isready -U litellm"], interval: 2s, retries: 30}
  idp:
    image: ghcr.io/berriai/litellm@sha256:1050d2c9ca58399de9cd51df6eb503812af697717f54d2f08c1c30589c75e1f7
    entrypoint: ["python3", "/idp.py"]
    volumes: ["$D/idp.py:/idp.py:ro"]
    environment:
      IDP_CLIENT_ID: zero-litellm
      IDP_CLIENT_SECRET: "\${SECRET}"
      IDP_REDIRECT: $BASE$ROOT/sso/callback
    ports: ["127.0.0.1:$IDP:8000"]
  models:
    image: ghcr.io/berriai/litellm@sha256:1050d2c9ca58399de9cd51df6eb503812af697717f54d2f08c1c30589c75e1f7
    command: ["--config", "/app/config.yaml", "--port", "4000"]
    volumes: ["$D/config.yaml:/app/config.yaml:ro"]
    depends_on: {db: {condition: service_healthy}}
    environment:
      SERVER_ROOT_PATH: $ROOT
      LITELLM_MASTER_KEY: "\${MASTER}"
      DATABASE_URL: postgresql://litellm:\${DBPW}@db:5432/litellm
      PROXY_BASE_URL: $BASE
      GENERIC_CLIENT_ID: zero-litellm
      GENERIC_CLIENT_SECRET: "\${SECRET}"
      GENERIC_AUTHORIZATION_ENDPOINT: http://127.0.0.1:$IDP/authorize
      GENERIC_TOKEN_ENDPOINT: http://idp:8000/token
      GENERIC_USERINFO_ENDPOINT: http://idp:8000/userinfo
      GENERIC_USER_ID_ATTRIBUTE: sub
      GENERIC_SCOPE: openid profile email
      PROXY_ADMIN_ID: zero-local
      AUTO_REDIRECT_UI_LOGIN_TO_SSO: "true"
    ports: ["127.0.0.1:$PORT:4000"]
YAML

dc up -d >"$D/up.log" 2>&1 || { tail -5 "$D/up.log"; echo "FAIL compose up"; exit 1; }
i=0
until [ "$(curl -s -o /dev/null -m 3 -w '%{http_code}' "$BASE$ROOT/health/liveliness")" = 200 ]; do
	i=$((i+1)); [ $i -lt 100 ] || { dc logs models 2>&1 | tail -5; echo "FAIL proxy not up after 5 min"; exit 1; }
	sleep 3
done
pass "proxy up with the OpenID settings"

# 1. sign-in: no typing; the provider approves, the proxy sets its cookie
end=$(curl -s -L -m 60 -c "$D/jar" -b "$D/jar" -o /dev/null -w '%{http_code} %{url_effective}' "$BASE$ROOT/sso/key/generate")
has "sign-in ends in the UI" "$end" "200 $BASE$ROOT/ui/.*login=success"
case $end in 200*) ;; *) echo "proxy log:"; dc logs models 2>&1 | grep -iE 'error|exception|Traceback|sso' | grep -viE 'secret|master|sk-' | tail -n 12 | cut -c1-260;; esac
case $end in *sk-*|*key=*|*token=*) fail "no key in the address" "$end";; *) pass "no key in the address";; esac
jwt=$(awk '$6 == "token" {print $7}' "$D/jar")
[ -n "$jwt" ] && pass "session cookie set" || fail "session cookie set" "no cookie named token"

# 2. who the proxy thinks this is (payload of its cookie; the key stays here)
python3 - "$jwt" > "$D/who" <<'PY' || true
import base64, json, sys
p = sys.argv[1].split(".")[1]
d = json.loads(base64.urlsafe_b64decode(p + "=" * (-len(p) % 4)))
print(d.get("user_id", ""), d.get("user_role", ""))
print(d.get("key", ""))
PY
is "signed in as the provider's user, as admin" "$(sed -n 1p "$D/who")" "zero-local proxy_admin"
key=$(sed -n 2p "$D/who")
code=$(printf 'header = "Authorization: Bearer %s"\n' "$key" | curl -s -K - -o /dev/null -m 20 -w '%{http_code}' "$BASE$ROOT/user/list")
is "the session's key works on an admin call" "$code" 200

# 3. typed login is off
code=$(curl -s -o /dev/null -m 20 -w '%{http_code}' -X POST -d 'username=admin' --data-urlencode "password=$(sed -n 's/^MASTER=//p' "$D/env")" "$BASE$ROOT/login")
case $code in 2*|3*) fail "typed login refused" "status $code";; *) pass "typed login refused ($code)";; esac

# 4. the provider: a code works once, the client secret is needed
loc=$(curl -s -o /dev/null -m 10 -w '%{redirect_url}' "http://127.0.0.1:$IDP/authorize?client_id=zero-litellm&state=x&redirect_uri=$BASE$ROOT/sso/callback")
c=$(printf %s "$loc" | sed 's/.*code=\([^&]*\).*/\1/')
tok() { printf 'user = "zero-litellm:%s"\n' "$1" | curl -s -K - -o /dev/null -m 10 -w '%{http_code}' -d "grant_type=authorization_code&code=$c" --data-urlencode "redirect_uri=$BASE$ROOT/sso/callback" "http://127.0.0.1:$IDP/token"; }
is "provider: wrong client secret refused" "$(tok wrong)" 401
is "provider: code accepted once" "$(tok "$(sed -n 's/^SECRET=//p' "$D/env")")" 200
is "provider: the same code again refused" "$(tok "$(sed -n 's/^SECRET=//p' "$D/env")")" 400
is "provider: other redirect address refused" "$(curl -s -o /dev/null -m 10 -w '%{http_code}' "http://127.0.0.1:$IDP/authorize?client_id=zero-litellm&state=x&redirect_uri=http://evil.invalid/")" 400

if [ -n "${HOLD:-}" ]; then echo "holding for a browser check: $BASE$ROOT/ui/ (enter to end)"; read -r _; fi
echo "fails: $fails"
[ $fails -eq 0 ]
