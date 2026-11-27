#!/bin/sh
# PoC 27 (#164): MCP servers declared in the model proxy's config, the tool
# list on a key, changed on the existing key; GitLab's MCP endpoint behind the
# proxy with a token that stays in the proxy.
# Needs: PoC 17's state (~/.local/state/zero-poc17: key, pg), docker compose
# (COMPOSE), glab logged in to FORGE_HOST, jq. KEEP=1 leaves the stack running.
# It mints one GitLab token (zero-poc27, read_project on PROJECT_ID) and
# revokes it at the end. No key or token is printed.
set -eu
cd "$(dirname "$0")"
COMPOSE=${COMPOSE:-docker compose}
FORGE_HOST=${FORGE_HOST:-gitlab.com}
PROJECT_ID=${PROJECT_ID:-88739432}
STATE=${XDG_STATE_HOME:-$HOME/.local/state}/zero-poc17
[ -f "$STATE/key" ] && [ -f "$STATE/pg" ] || { echo "FAIL no state of PoC 17 in $STATE"; exit 1; }
[ -f "$STATE/mcp" ] || (umask 077; head -c 24 /dev/urandom | base64 | tr -d '/+=' > "$STATE/mcp")
export LITELLM_MASTER_KEY="$(cat "$STATE/key")" COPILOT_DIR="$STATE/copilot" PG_PASSWORD="$(cat "$STATE/pg")"
export MCP_UPSTREAM_TOKEN="$(cat "$STATE/mcp")" POC18="$(cd ../18-litellm-sandbox && pwd)" POC27="$(pwd)"
C="$COMPOSE -f ../17-litellm/compose.yaml -f ../17-litellm/compose.ui.yaml -f ../18-litellm-sandbox/compose.yaml -f compose.yaml"
P=http://127.0.0.1:4000
W=$(mktemp -d)
TID=
ALIASES='"poc27-a","poc27-none","poc27-g"'

glapi() { glab api "$@" --hostname "$FORGE_HOST"; }
auth() { printf 'header = "Authorization: Bearer %s"\n' "$1"; }
adm() { auth "$LITELLM_MASTER_KEY" | curl -s -K - "$@"; }
api() { adm -H 'content-type: application/json' "$P$1" -d "$2"; }

cleanup() {
  api /key/delete "{\"key_aliases\":[$ALIASES]}" >/dev/null 2>&1 || true
  if [ -n "$TID" ]; then glapi -X DELETE "/personal_access_tokens/$TID" >/dev/null 2>&1 || echo "CLEANUP FAILED: GitLab token $TID not revoked"; fi
  [ "${KEEP:-}" = 1 ] || $C down >/dev/null 2>&1 || true
  rm -rf "$W"
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
is() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "got $2, want $3"; fi; }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
hasnot() { if [ -n "$2" ] && ! printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }

# GitLab token for the proxy
body=$(jq -n --arg e "$(date -u -d tomorrow +%F)" --argjson id "$PROJECT_ID" \
  '{name:"zero-poc27", expires_at:$e, granular_scopes:[{access:"selected_memberships", permissions:["read_project"], project_ids:[$id]}, {access:"user", permissions:["execute_mcp_tool"]}]}')
out=$(printf %s "$body" | glapi -X POST /user/personal_access_tokens -H 'Content-Type: application/json' --input -)
TID=$(printf %s "$out" | jq -er .id) || { echo "FAIL GitLab token not minted"; exit 1; }
export GITLAB_MCP_TOKEN="$(printf %s "$out" | jq -er .token)"
out=

$C up -d > "$W/up.log" 2>&1 || { grep -v -e "$LITELLM_MASTER_KEY" -e "$GITLAB_MCP_TOKEN" "$W/up.log"; echo "FAIL compose up"; exit 1; }
i=0
until [ "$(curl -s -o /dev/null -w '%{http_code}' "$P/health/liveliness")" = 200 ]; do
  i=$((i+1)); [ $i -lt 150 ] || { echo "FAIL proxy not up after 300 s"; exit 1; }; sleep 2
done

key() { k=$(api /key/generate "{\"key_alias\":\"poc27-$1\",\"duration\":\"1h\",$2}" | jq -r .key); [ -n "$k" ] && [ "$k" != null ] || { echo "FAIL no key $1" >&2; kill $$; }; echo "$k"; }
# mcp HEADERNAME KEY METHOD PARAMS
mcp() { printf 'header = "%s: Bearer %s"\n' "$1" "$2" | curl -s -K - "$P/mcp" -H 'content-type: application/json' -H 'accept: application/json, text/event-stream' \
  -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$3\",\"params\":$4}" | sed -n 's/^data: //p'; }
tools() { mcp x-litellm-api-key "$1" tools/list '{}' | jq -c '[.result.tools[].name]'; }
ADMIN='"user_id":"default_user_id","models":["claude-stand-in"]'

echo "== tool list on a key, changed on the existing key"
A=$(key a "$ADMIN,\"object_permission\":{\"mcp_servers\":[\"notes\"],\"mcp_tool_permissions\":{\"notes\":[\"read_note\"]}}")
N=$(key none "$ADMIN,\"object_permission\":{\"mcp_servers\":[\"no-mcp-servers\"]}")
is "key with a list: that tool only" "$(tools "$A")" '["notes-read_note"]'
is "key with no-mcp-servers: no tool" "$(tools "$N")" '[]'
is "the Authorization header works for /mcp too" "$(mcp Authorization "$A" tools/list '{}' | jq -c '[.result.tools[].name]')" '["notes-read_note"]'
has "tool outside the list refused" "$(mcp x-litellm-api-key "$A" tools/call '{"name":"notes-write_note","arguments":{"text":"x"}}')" "not allowed"
u=$(api /key/update "{\"key\":\"$A\",\"object_permission\":{\"mcp_servers\":[\"notes\"],\"mcp_tool_permissions\":{\"notes\":[\"read_note\",\"write_note\"]}}}" | jq -r 'has("key") or has("key_name") or has("token")')
is "key update accepted" "$u" true
is "next request: the added tool is listed" "$(tools "$A")" '["notes-read_note","notes-write_note"]'
has "next request: the added tool answers" "$(mcp x-litellm-api-key "$A" tools/call '{"name":"notes-write_note","arguments":{"text":"x"}}')" "write_note: x"
api /key/update "{\"key\":\"$A\",\"object_permission\":{\"mcp_servers\":[\"no-mcp-servers\"],\"mcp_tool_permissions\":{}}}" >/dev/null
is "list taken away: next request no tool" "$(tools "$A")" '[]'
has "list taken away: call refused" "$(mcp x-litellm-api-key "$A" tools/call '{"name":"notes-read_note","arguments":{"text":"x"}}')" "rror"

echo "== GitLab's MCP endpoint behind the proxy"
G=$(key g "$ADMIN,\"object_permission\":{\"mcp_servers\":[\"gitlab\"],\"mcp_tool_permissions\":{\"gitlab\":[\"get_project\"]}}")
is "key limited to get_project: that tool only" "$(tools "$G")" '["gitlab-get_project"]'
r=$(mcp x-litellm-api-key "$G" tools/call "{\"name\":\"gitlab-get_project\",\"arguments\":{\"project_id\":\"$PROJECT_ID\"}}")
has "get_project answers through the proxy" "$r" "\"id\\\\*\":[ ]*$PROJECT_ID"
hasnot "answer holds no GitLab token" "$r" "$GITLAB_MCP_TOKEN"
has "another GitLab tool refused by the proxy" "$(mcp x-litellm-api-key "$G" tools/call "{\"name\":\"gitlab-list_commits\",\"arguments\":{\"project_id\":\"$PROJECT_ID\"}}")" "not allowed"
is "key without the gitlab server: none of its tools" "$(tools "$N")" '[]'

echo "== what the proxy records for a tool call"
sleep 3
s=$(adm "$P/spend/logs?start_date=$(date -u +%F)&end_date=$(date -u -d tomorrow +%F)" | jq -c '[.[]? | select((.call_type // "") | test("mcp"))] | length' 2>/dev/null || echo "?")
echo "NOTE spend log rows with an mcp call type: $s"
l=$($C logs litellm 2>&1)
if [ -n "$l" ] && ! printf %s "$l" | grep -q -e "$LITELLM_MASTER_KEY" -e "$GITLAB_MCP_TOKEN" -e "$MCP_UPSTREAM_TOKEN"; then pass "no key or token in the proxy log"; else fail "no key or token in the proxy log" "found or log empty"; fi

echo "fails: $fails"
[ $fails -eq 0 ]
