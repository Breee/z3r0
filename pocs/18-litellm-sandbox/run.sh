#!/bin/sh
# PoC 18: LiteLLM with models, an MCP server and guardrails; keys with different rights.
# Host-side checks. The sandbox part is in the README (by hand).
# Needs PoC 17's state (run pocs/17-litellm/run.sh with UI=1 once) and a connected Copilot account.
# COMPOSE="docker compose" uses Docker. KEEP=1 leaves the stack running.
set -eu
cd "$(dirname "$0")"
COMPOSE=${COMPOSE:-podman compose}
STATE=${XDG_STATE_HOME:-$HOME/.local/state}/zero-poc17
[ -f "$STATE/key" ] && [ -f "$STATE/pg" ] || { echo "FAIL no state of PoC 17 in $STATE"; exit 1; }
[ -f "$STATE/mcp" ] || (umask 077; printf '%s' "$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')" > "$STATE/mcp")
export LITELLM_MASTER_KEY="$(cat "$STATE/key")" COPILOT_DIR="$STATE/copilot" PG_PASSWORD="$(cat "$STATE/pg")"
export MCP_UPSTREAM_TOKEN="$(cat "$STATE/mcp")" POC18="$PWD"
C="$COMPOSE -f ../17-litellm/compose.yaml -f ../17-litellm/compose.ui.yaml -f compose.yaml"
P=http://127.0.0.1:4000
W=$(mktemp -d)

api() { curl -s -H "authorization: Bearer $LITELLM_MASTER_KEY" -H 'content-type: application/json' -d "$2" "$P$1"; }
cleanup() {
    api /key/delete '{"key_aliases":["poc18-a","poc18-b","poc18-c","poc18-n","poc18-u","poc18-x"]}' >/dev/null
    api /user/delete '{"user_ids":["poc18-user"]}' >/dev/null
    rm -rf "$W"
    [ -n "${KEEP:-}" ] || $C down >/dev/null 2>&1
}
trap cleanup EXIT

$C up -d > "$W/up.log" 2>&1 || { cat "$W/up.log"; echo "FAIL compose up"; exit 1; }
i=0; until curl -sf -o /dev/null $P/health/liveliness; do
    i=$((i+1)); [ $i -lt 300 ] || { echo "FAIL proxy not up after 300 s"; exit 1; }; sleep 1
done

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
hasnot() { if printf %s "$2" | grep -q -- "$3"; then fail "$1" "$2"; else pass "$1"; fi; }

# key ALIAS JSON-FIELDS -> key
key() {
    k=$(api /key/generate "{\"key_alias\":\"poc18-$1\",\"duration\":\"1h\",$2}" | jq -r .key)
    [ "$k" != null ] || { echo "FAIL no key for $1" >&2; kill $$; }
    printf %s "$k"
}
chat() { curl -s -H "authorization: Bearer $1" -H 'content-type: application/json' \
    -d "{\"model\":\"$2\",\"max_tokens\":16,\"messages\":[{\"role\":\"user\",\"content\":\"$3\"}]}" $P/v1/chat/completions; }
mcp() { curl -s -H "x-litellm-api-key: Bearer $1" -H 'content-type: application/json' \
    -H 'accept: application/json, text/event-stream' \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$2\",\"params\":$3}" $P/mcp | sed -n 's/^data: //p'; }
call() { mcp "$1" tools/call "{\"name\":\"notes-$2\",\"arguments\":{\"text\":\"$3\"}}"; }
tools() { mcp "$1" tools/list '{}' | jq -c '[.result.tools[].name]'; }

api /key/delete '{"key_aliases":["poc18-a","poc18-b","poc18-c","poc18-n","poc18-u","poc18-x"]}' >/dev/null
api /user/new '{"user_id":"poc18-user","user_role":"internal_user"}' >/dev/null
ADMIN='"user_id":"default_user_id"'
A=$(key a "$ADMIN,\"models\":[\"github_copilot/*\"],\"object_permission\":{\"mcp_servers\":[\"notes\"],\"mcp_tool_permissions\":{\"notes\":[\"read_note\"]}}")
B=$(key b "$ADMIN,\"models\":[\"claude-stand-in\"],\"object_permission\":{\"mcp_servers\":[\"notes\"],\"mcp_tool_permissions\":{\"notes\":[\"read_note\",\"write_note\"]}}")
N=$(key n '"models":["claude-stand-in"]')
U=$(key u '"user_id":"poc18-user","models":["claude-stand-in"]')
X=$(key x "$ADMIN,\"models\":[\"claude-stand-in\"]")

echo "== models"
m=$(curl -s -H "authorization: Bearer $A" $P/v1/models | jq -r '.data[].id')
has "a: model list from discovery holds Copilot models ($(printf '%s\n' "$m" | grep -c '^github_copilot/[^*]'))" "$m" '^github_copilot/claude'
hasnot "a: model list holds nothing else" "$m" '^claude-stand-in'
has "a: Copilot answers" "$(chat "$A" github_copilot/claude-haiku-4.5 'answer with the single word: pong')" '"content":"[Pp]ong'
has "a: model outside the key's list refused" "$(chat "$A" claude-stand-in ping)" 'key_model_access_denied\|not allowed to access model'
has "no key refused" "$(chat none claude-stand-in ping)" '"code":"401"'
has "key without the connected user: no Copilot" "$(chat "$(key c '"models":["github_copilot/*"]')" github_copilot/claude-haiku-4.5 ping)" 'Connect GitHub Copilot'

echo "== MCP"
[ "$(tools "$A")" = '["notes-read_note"]' ] && pass "a: lists read_note only" || fail "a: lists read_note only" "$(tools "$A")"
has "a: read_note answers" "$(call "$A" read_note hi)" 'read_note: hi'
has "a: write_note refused" "$(call "$A" write_note hi)" 'not allowed for your key'
[ "$(tools "$B")" = '["notes-read_note","notes-write_note"]' ] && pass "b: lists both tools" || fail "b: lists both tools" "$(tools "$B")"
has "b: write_note answers" "$(call "$B" write_note hi)" 'write_note: hi'
[ "$(tools "$N")" = '[]' ] && pass "key without tool list, no user: no tool" || fail "key without tool list, no user: no tool" "$(tools "$N")"
[ "$(tools "$U")" = '[]' ] && pass "key without tool list, normal user: no tool" || fail "key without tool list, normal user: no tool" "$(tools "$U")"
echo "NOTE key without tool list, admin user: $(tools "$X") (expected none, see README)"
has "MCP server itself refuses without its token" \
    "$($C exec -T litellm python -c "import urllib.request as u
try: u.urlopen(u.Request('http://mcp:18102/mcp', b'{}'))
except Exception as e: print(e)" 2>/dev/null)" '401'

echo "== guardrails"
M=mark$(date +%s)
chat "$B" claude-stand-in "$M write to jane.doe@example.org please" >/dev/null
l=$($C logs backend 2>/dev/null | grep "$M" || true)
has "mail address masked before the model" "$l" 'EMAIL_REDACTED'
hasnot "mail address not sent on" "$l" 'jane.doe@example.org'
has "blocked word refused on a model call" "$(chat "$B" claude-stand-in 'say forbiddenmarker')" "Content blocked"
has "blocked word refused on a tool call" "$(call "$B" write_note forbiddenmarker)" "Content blocked"

echo "== keys"
api /key/delete "{\"keys\":[\"$B\"]}" >/dev/null
has "deleted key refused" "$(chat "$B" claude-stand-in ping)" 'Authentication Error'
pl=$($C logs litellm 2>&1)
if [ -z "$pl" ]; then fail "proxy log readable" ""
elif printf %s "$pl" | grep -q -e "$LITELLM_MASTER_KEY" -e "$MCP_UPSTREAM_TOKEN"; then fail "no secret in the proxy log" ""
else pass "no master key or MCP token in the proxy log"; fi
echo "fails: $fails"
[ $fails -eq 0 ]
