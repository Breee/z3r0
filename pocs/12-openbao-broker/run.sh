#!/usr/bin/env bash
# PoC 12 (#98): GitLab web login into OpenBao; the user's GitLab OAuth refresh token is stored in
# OpenBao and used to mint and revoke a fine-grained per-sandbox token.
# Interactive: opens the browser twice (OpenBao login, GitLab authorization).
# Needs: bao, glab (logged in), jq, curl, git, python3, a browser.
# Usage: [BAO=<bao binary>] [PROJECT=<group/project on gitlab.com>] run.sh
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
BAO=${BAO:-bao}
PROJECT=${PROJECT:-corewire/technology-and-research/developer}
REDIRECT=http://localhost:8250/oidc/callback
API=https://gitlab.com/api/v4
umask 077
W=$(mktemp -d)
FAILED=0
export BAO_ADDR=http://127.0.0.1:8200 BAO_TOKEN=root-poc

pass() { echo "PASS $1"; }
fail() { echo "FAIL $1"; FAILED=1; }
check() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1 (got $2, want $3)"; fi; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

cleanup() {
    [ -n "${CHILD_ID:-}" ] && glab api -X DELETE "personal_access_tokens/$CHILD_ID" >/dev/null 2>&1
    [ -n "${APP_NUM:-}" ] && glab api -X DELETE "user/applications/$APP_NUM" >/dev/null 2>&1
    kill "${BP:-}" 2>/dev/null
    rm -rf "$W"
}
trap cleanup EXIT

GL_UID=$(glab api user | jq -r .id)
PID_=$(glab api "projects/$(printf %s "$PROJECT" | jq -sRr @uri)" | jq -r .id)

# GitLab OAuth application owned by the user (no admin needed).
glab api -X POST user/applications -f name=zero-poc12 -f redirect_uri=$REDIRECT \
    -f scopes="openid api" -f confidential=false >"$W/app.json"
APP_NUM=$(jq -r .id "$W/app.json"); APP=$(jq -r .application_id "$W/app.json")

# OpenBao dev server with a file audit device (audit devices are config-only in OpenBao 2.7).
printf 'audit "file" "file" {\n  options = {\n    file_path = "%s/audit.log"\n  }\n}\n' "$W" >"$W/audit.hcl"
"$BAO" server -dev -dev-root-token-id=root-poc -dev-listen-address=127.0.0.1:8200 -config="$W/audit.hcl" >"$W/bao.log" 2>&1 &
BP=$!
for _ in $(seq 50); do "$BAO" status >/dev/null 2>&1 && break; sleep 0.1; done

"$BAO" auth enable oidc >/dev/null
printf %s "$(jq -r .secret "$W/app.json")" >"$W/secret"
"$BAO" write auth/oidc/config oidc_discovery_url=https://gitlab.com oidc_client_id="$APP" \
    oidc_client_secret=@"$W/secret" default_role=zero >/dev/null
ACC=$("$BAO" auth list -format=json | jq -r '."oidc/".accessor')
"$BAO" policy write zero-user - >/dev/null <<POL
path "secret/data/zero/users/{{identity.entity.aliases.$ACC.name}}/*" {
  capabilities = ["create", "read", "update", "delete"]
}
POL
role() { "$BAO" write "auth/oidc/role/$1" role_type=oidc user_claim=sub bound_subject="$2" bound_audiences="$APP" \
    allowed_redirect_uris=$REDIRECT oidc_scopes=openid token_policies=zero-user token_ttl=1h >/dev/null; }
role zero "$GL_UID"
role other 1

# (a) GitLab web login gives an OpenBao token bound to the GitLab user.
echo ">> browser 1/2: OpenBao login via GitLab"
BAO_TOKEN= "$BAO" login -method=oidc -no-store -format=json role=zero >"$W/login.json" 2>"$W/login.err"
check "GitLab web login yields an OpenBao token" $? 0
UT=$(jq -r '.auth.client_token // empty' "$W/login.json")
[ -n "$UT" ] || { cat "$W/login.err"; echo "FAILED (no OpenBao token, stopping)"; exit 1; }
BAO_TOKEN= "$BAO" login -method=oidc -no-store -format=json role=other >/dev/null 2>"$W/other.err"
check "role bound to another GitLab user is refused" $? 2
BAO_TOKEN=$UT "$BAO" kv put -mount=secret "zero/users/$GL_UID/probe" v=1 >/dev/null 2>&1
check "user writes own path" $? 0
BAO_TOKEN=$UT "$BAO" kv put -mount=secret zero/users/1/probe v=1 >/dev/null 2>&1
check "user denied on another user's path" $? 2

# (b) refresh token in OpenBao -> access token -> fine-grained child token.
echo ">> browser 2/2: authorize GitLab API access"
python3 "$HERE/oauth.py" "$APP" "$BAO_ADDR" "$UT" "$GL_UID"
check "refresh token stored in OpenBao" $? 0
getrt() { BAO_TOKEN=$UT "$BAO" kv get -mount=secret -field=refresh_token "zero/users/$GL_UID/gitlab"; }
RT=$(getrt)
curl -s https://gitlab.com/oauth/token -d grant_type=refresh_token -d client_id="$APP" \
    --data-urlencode "refresh_token=$RT" -d redirect_uri=$REDIRECT >"$W/tok.json"
AT=$(jq -r .access_token "$W/tok.json")
check "refresh gives a 2h access token" "$(jq -r .expires_in "$W/tok.json")" 7200
BAO_TOKEN=$UT "$BAO" kv put -mount=secret "zero/users/$GL_UID/gitlab" refresh_token="$(jq -r .refresh_token "$W/tok.json")" >/dev/null
[ "$(getrt)" != "$RT" ] && pass "rotated refresh token stored back" || fail "rotated refresh token stored back"
check "old refresh token is dead" "$(curl -s https://gitlab.com/oauth/token -d grant_type=refresh_token -d client_id="$APP" \
    --data-urlencode "refresh_token=$RT" -d redirect_uri=$REDIRECT | jq -r .error)" invalid_grant

jq -n --argjson p "$PID_" --arg e "$(date -u -d tomorrow +%F)" '{name:"zero-poc12", expires_at:$e, granular_scopes:[{access:"selected_memberships",
  permissions:["read_project","read_code","download_code","push_code","create_merge_request","read_merge_request","read_job","read_pipeline"],
  project_ids:[$p]}]}' | curl -s -X POST "$API/user/personal_access_tokens" -H "Authorization: Bearer $AT" \
    -H 'Content-Type: application/json' --data @- >"$W/child.json"
CHILD_ID=$(jq -r .id "$W/child.json"); CT=$(jq -r .token "$W/child.json")
check "child token is fine-grained" "$(jq -r .granular "$W/child.json")" true
check "child reads the project" "$(code -H "Authorization: Bearer $CT" "$API/projects/$PID_")" 200
check "child cannot read /user" "$(code -H "Authorization: Bearer $CT" "$API/user")" 403
git ls-remote "https://oauth2:$CT@gitlab.com/$PROJECT.git" >/dev/null 2>&1
check "child works for git" $? 0
check "child revoked with the access token" "$(code -X DELETE -H "Authorization: Bearer $AT" "$API/personal_access_tokens/$CHILD_ID")" 204
check "revoked child is refused" "$(code -H "Authorization: Bearer $CT" "$API/projects/$PID_")" 401
CHILD_ID=

# (f) audit: who touched the root credential.
N=$(jq -r --arg p "secret/data/zero/users/$GL_UID/gitlab" 'select(.type=="request" and .request.path==$p) | .auth.display_name' "$W/audit.log" | grep -c "^oidc-$GL_UID$")
[ "$N" -ge 3 ] && pass "audit log names the GitLab user on each root credential access ($N)" || fail "audit log names the GitLab user (got $N)"
jq -c --arg p "secret/data/zero/users/$GL_UID/gitlab" 'select(.type=="request" and .request.path==$p) | {time, user:.auth.display_name, op:.request.operation, path:.request.path}' "$W/audit.log"

[ $FAILED = 0 ] && echo "ALL PASS" || echo "FAILED"
exit $FAILED
