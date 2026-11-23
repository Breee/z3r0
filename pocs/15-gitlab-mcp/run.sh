#!/usr/bin/env bash
# PoC 15 (#130): one agentgateway for all workspaces, GitLab's instance MCP endpoint behind it.
# zero-cp answers per request with the workspace's GitLab token and tool list.
# Needs the SPIRE + gateway environment of PoC 08 (see README.md), podman, openshell, glab (logged in to FORGE_HOST).
# Usage: OPENSHELL_SRC=<OpenShell v0.1.8 clone> [FORGE_HOST=gitlab.com] [PROJECT_A=<group/project>] [PROJECT_B=<group/project>]
#        [IMAGE=<image with curl>] run.sh
set -u
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HERE=$ROOT/pocs/15-gitlab-mcp
SPIRE=${OPENSHELL_SRC:?set OPENSHELL_SRC}/examples/spiffe-token-exchange-demo/podman/spire
HOST=${FORGE_HOST:-gitlab.com}
PA=${PROJECT_A:-corewire/technology-and-research/developer}
PB=${PROJECT_B:-corewire/technology-and-research/developer2}
IMAGE=${IMAGE:-$(sed -n 's/^const devImage = "\(.*\)"/\1/p' "$ROOT/internal/zero/zero.go")}
AGIMG=ghcr.io/agentgateway/agentgateway@sha256:460dda874be2dfa95fa5713c1f46f26079dce78df576008df704e54a59d5780d # v1.7.0
O="openshell --gateway spiffe-poc --gateway-endpoint http://127.0.0.1:18082"
URL=http://host.openshell.internal:18091/mcp
PRE=spiffe://openshell.local/openshell/sandbox
W=$(mktemp -d)
FAILED=0

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
names() { mcp "$1" tools/list '{}' | grep -o '"name":"[a-z_]*"' | cut -d'"' -f4 | sort | tr '\n' ' ' | sed 's/ $//'; }
want() { tr -s ' \n' '\n\n' | sort | tr '\n' ' ' | sed 's/^ //; s/ $//'; }

# what zero does on `zero up`: a token for the workspace's projects with the permissions of git.read, usable for MCP
mint() {
    local pid out
    pid=$(glab api "projects/$(printf %s "$2" | jq -sRr @uri)" --hostname "$HOST" | jq -r .id) || return 1
    out=$(jq -n --arg n "zero-poc15-$1" --argjson p "$pid" '{name: $n, expires_at: (now + 86400 | strftime("%Y-%m-%d")),
        granular_scopes: [{access: "selected_memberships", permissions: ["read_project", "read_code", "download_code"], project_ids: [$p]},
                          {access: "user", permissions: ["execute_mcp_tool"]}]}' |
        glab api -X POST /user/personal_access_tokens -H 'Content-Type: application/json' --input - --hostname "$HOST") || return 1
    printf %s "$out" | jq -r .id >> "$W/tokids"
    printf %s "$out" | jq -r .token
}
gateway() { # gateway RULE: start the gateway with one mcpAuthorization rule
    podman logs poc15-gateway >> "$W/gateway.log" 2>&1
    podman rm -f poc15-gateway >/dev/null 2>&1
    sed -e "s#ISSUER#http://spire-oidc:8080#" -e "s#MCPURL#https://$HOST/api/v4/mcp#" -e "s#RULE#$1#" "$HERE/gateway.yaml" > "$W/gateway.yaml"
    podman run -d --name poc15-gateway --network host --user 0 -v "$W:/w" "$AGIMG" -f /w/gateway.yaml >/dev/null || exit 1
    sleep 8
}

cleanup() {
    podman rm -f poc15-gateway poc15-zero-cp >/dev/null 2>&1
    for id in $(cat "$W/tokids" 2>/dev/null); do glab api -X DELETE "/personal_access_tokens/$id" --hostname "$HOST" >/dev/null 2>&1; done
    $O sandbox delete gl-a gl-b >/dev/null 2>&1
    sleep 2; $O provider delete zero-mcp-poc >/dev/null 2>&1
    [ -n "${KEEP:-}" ] && { rm -rf "$W/sessions"; cp -r "$W" "$KEEP"; }
    rm -rf "$W"
}
trap cleanup EXIT

podman build -q -t localhost/poc15-zero-cp -f "$HERE/Containerfile" "$ROOT" >/dev/null || exit 1
podman exec openshell-spiffe-demo-spire-server /opt/spire/bin/spire-server bundle show \
    -format spiffe -socketPath /run/spire/server/private/api.sock > "$W/bundle.json" || exit 1
jq '{keys: [.keys[] | select(.use == "jwt-svid") | .use = "sig"]}' "$W/bundle.json" > "$W/jwks.json"
: > "$W/allow.txt"; mkdir "$W/sessions" "$W/tools"
(umask 077; head -c 32 /dev/urandom | basenc --base64url -w0 > "$W/gateway.key")
podman rm -f poc15-zero-cp >/dev/null 2>&1
podman run -d --name poc15-zero-cp --network host -v "$W:/w" localhost/poc15-zero-cp -svid -jwks /w/bundle.json -audience zero-mcp \
    -allow /w/allow.txt -sessions /w/sessions -tools /w/tools -gateway-key /w/gateway.key -log /w/decisions.jsonl >/dev/null || exit 1

$O profile import --file "$ROOT/pocs/13-agentgateway/profile.yaml" >/dev/null 2>&1
$O provider create --name zero-mcp-poc --type zero-mcp-poc --runtime-credentials >/dev/null || exit 1
for n in a b; do
    $O sandbox create --name gl-$n --from "$IMAGE" --provider zero-mcp-poc --detach >/dev/null || exit 1
done
sleep 10
for n in a b; do
    id=$($O sandbox get gl-$n --output json | jq -r .id)
    bash "$SPIRE/register-sandbox.sh" "$id" >/dev/null || exit 1
    eval "SB_$n=$id"
    echo "$PRE/$id" >> "$W/allow.txt"
done
(umask 077; mint a "$PA" > "$W/sessions/$SB_a" && mint b "$PB" > "$W/sessions/$SB_b") || exit 1
[ -s "$W/sessions/$SB_a" ] && [ -s "$W/sessions/$SB_b" ] || { echo "mint failed"; exit 1; }
TOKA=$(cat "$W/sessions/$SB_a")
# what zero writes from its config: workspace a overrides the tool list, workspace b has none and gets the default
OVERRIDE="get_project list_branches"
echo "$OVERRIDE" > "$W/tools/$SB_a"
cp "$HERE/default-tools.txt" "$W/tools/$SB_b"
PAJ="{\"project_id\":\"$PA\"}"; PBJ="{\"project_id\":\"$PB\"}"

echo "--- variant B: zero-cp answers with the tool list per request"
gateway "'mcp.tool.name in extauthz.tools.split(\",\")'"
check "a: tools/list is exactly the override" "$(names gl-a)" "$(echo "$OVERRIDE" | want)"
check "b: same gateway, tools/list is exactly the default" "$(names gl-b)" "$(want < "$HERE/default-tools.txt")"
RA=$(call gl-a get_project "$PAJ")
has "a: get_project on a's project" "$RA" "${PA##*/}"
hasnot "a: get_project is no error" "$RA" '"isError":true'
has "a: tool outside the list refused by the gateway (list_commits)" "$(call gl-a list_commits "{\"id\":\"$PA\"}")" '"error"'
has "a: write tool refused by the gateway (save_note)" "$(call gl-a save_note "$PAJ")" '"error"'
has "a: b's project refused by GitLab (a's token)" "$(call gl-a get_project "$PBJ")" 'Access denied\|isError'
RB=$(call gl-b get_project "$PBJ")
has "b: get_project on b's project (b's token)" "$RB" "${PB##*/}"
hasnot "b: get_project is no error" "$RB" '"isError":true'
has "b: a's project refused by GitLab (b's token)" "$(call gl-b get_project "$PAJ")" 'Access denied\|isError'
has "b: default tool works (list_commits)" "$(call gl-b list_commits "$PBJ")" 'gid://gitlab/Commit/'
has "b: tool in the list but beyond the token's permissions refused by GitLab (list_branches)" \
    "$(call gl-b list_branches "{\"id\":\"$PB\"}")" 'insufficient_granular_scope'
check "no identity (host, not through the supervisor): 401" \
    "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'content-type: application/json' -d '{}' 127.0.0.1:18091/mcp)" 401
check "GitLab token as identity at the gateway: 401" \
    "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H "authorization: Bearer $TOKA" -H 'content-type: application/json' -d '{}' 127.0.0.1:18091/mcp)" 401
check "subject header without the gateway key at zero-cp: 403" \
    "$(curl -s -o /dev/null -w '%{http_code}' -H "x-zero-subject: $PRE/$SB_a" 127.0.0.1:18090/authz/)" 403
SEEN=$($O sandbox exec --name gl-a -- sh -c \
    'env; curl -s -m 30 -D- -X POST -H "content-type: application/json" -d "{}" '$URL'; grep -rs "glpat-" /tmp /sandbox /proc/[0-9]*/environ' | grep -c 'glpat-')
check "a: no GitLab token in the sandbox (env, files, response)" "$SEEN" 0
hasnot "a: tool results carry no token" "$RA" 'glpat-'
check "a: GitLab not reachable from the sandbox directly" \
    "$($O sandbox exec --name gl-a -- sh -c "curl -s -m 10 -o /dev/null -w '%{http_code}' https://$HOST/api/v4/mcp" 2>/dev/null | tail -c 3)" 000

echo get_project > "$W/tools/$SB_a"
check "a: tool removed from the list: gone at the next request" "$(names gl-a)" get_project
has "a: removed tool refused by the gateway" "$(call gl-a list_branches "{\"id\":\"$PA\"}")" '"error"'
rm "$W/tools/$SB_a"
check "a: no tool list: no tool" "$(names gl-a)" ""
has "a: no tool list: call refused" "$(call gl-a get_project "$PAJ")" '"error"'
rm "$W/sessions/$SB_a"
has "a: session removed in zero-cp: gateway fails closed" "$(mcp gl-a tools/list '{}')" 'no session'
has "b: still works after a's session ended" "$(call gl-b get_project "$PBJ")" "${PB##*/}"

echo "--- variant A: zero writes one rule per workspace into the gateway config"
(umask 077; printf %s "$TOKA" > "$W/sessions/$SB_a")
gateway "'(jwt.sub == \"$PRE/$SB_a\" \&\& mcp.tool.name in [\"get_project\", \"list_branches\"]) || (jwt.sub == \"$PRE/$SB_b\" \&\& mcp.tool.name in [\"get_project\"])'"
check "A: a sees its tools" "$(names gl-a)" "get_project list_branches"
check "A: b sees its tools" "$(names gl-b)" "get_project"
sed "s#\"get_project\", \"list_branches\"#\"get_project\"#" "$W/gateway.yaml" > "$W/gateway.new"; cat "$W/gateway.new" > "$W/gateway.yaml"
sleep 5
check "A: rule changed in the config file: picked up without restart" "$(names gl-a)" "get_project"

echo "--- gateway log (tool calls)"
podman logs poc15-gateway >> "$W/gateway.log" 2>&1
grep 'mcp.method.name=tools/call' "$W/gateway.log" | cut -f3- | cut -c1-600 | head -n 4
check "gateway log carries no GitLab token" "$(grep -c 'glpat-' "$W/gateway.log" "$W/decisions.jsonl" | cut -d: -f2 | paste -sd+ | bc)" 0
echo "--- zero-cp decisions"
tail -n 4 "$W/decisions.jsonl" | cut -c1-300
exit $FAILED
