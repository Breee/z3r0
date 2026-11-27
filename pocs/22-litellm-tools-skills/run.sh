#!/bin/sh
# PoC 22 (#172): tool lists and skills per key through LiteLLM, run.
# Needs: PoC 17's state (~/.local/state/zero-poc17: key, pg), docker compose, jq, zip, unzip,
# a sandbox of a workspace that uses zero's model proxy (for the last part).
# Env: COMPOSE (default "docker compose", ADR 0051), SANDBOX (default zero-developer),
# MODEL, KEEP=1 leaves the stack running. No key is printed.
set -eu
cd "$(dirname "$0")"
COMPOSE=${COMPOSE:-docker compose}
SANDBOX=${SANDBOX:-zero-developer}
MODEL=${MODEL:-github_copilot/claude-haiku-4.5}
STATE=${XDG_STATE_HOME:-$HOME/.local/state}/zero-poc17
[ -s "$STATE/key" ] && [ -s "$STATE/pg" ] || { echo "FAIL no state of PoC 17: run pocs/17-litellm/run.sh with UI=1 first"; exit 1; }
[ -s "$STATE/mcp" ] || (umask 077; head -c 24 /dev/urandom | base64 | tr -d '/+=' > "$STATE/mcp")
export LITELLM_MASTER_KEY="$(cat "$STATE/key")" COPILOT_DIR="$STATE/copilot" PG_PASSWORD="$(cat "$STATE/pg")"
export MCP_UPSTREAM_TOKEN="$(cat "$STATE/mcp")" POC18="$(cd ../18-litellm-sandbox && pwd)"
C="$COMPOSE -f ../17-litellm/compose.yaml -f ../17-litellm/compose.ui.yaml -f ../18-litellm-sandbox/compose.yaml"
P=http://127.0.0.1:4000
SKILL=poc22-skill
W=$(mktemp -d)
SSH="ssh -F $HOME/.local/state/zero/$SANDBOX/ssh.conf openshell-$SANDBOX.default"
sbx() { $SSH "$1" </dev/null 2>&1; }

# auth KEY: curl config on stdin, so that no key is a command argument.
auth() { printf 'header = "Authorization: Bearer %s"\n' "$1"; }
adm() { auth "$LITELLM_MASTER_KEY" | curl -s -K - "$@"; }
api() { adm -H 'content-type: application/json' "$P$1" -d "$2"; }
as() { k=$1; shift; auth "$k" | curl -s -K - "$@"; }

cleanup() {
  adm -o /dev/null -X DELETE "$P/skill-hub/skills/$SKILL" || true
  api /key/delete '{"key_aliases":["poc22-list","poc22-none","poc22-bare","poc22-with","poc22-without"]}' >/dev/null 2>&1 || true
  $SSH 'rm -rf /tmp/poc22' </dev/null >/dev/null 2>&1 || true
  rm -rf "$W"
  [ "${KEEP:-}" = 1 ] || $C down >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

$C up -d > "$W/up.log" 2>&1 || { cat "$W/up.log"; echo "FAIL compose up"; exit 1; }
i=0; until curl -sf -o /dev/null "$P/health/liveliness"; do
  i=$((i+1)); [ $i -lt 150 ] || { echo "FAIL proxy not up after 300 s"; exit 1; }; sleep 2
done
cleanup_before=$(api /key/delete '{"key_aliases":["poc22-list","poc22-none","poc22-bare","poc22-with","poc22-without"]}' || true)
adm -o /dev/null -X DELETE "$P/skill-hub/skills/$SKILL" || true

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
is() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "got $2, want $3"; fi; }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }

key() {
  k=$(api /key/generate "{\"key_alias\":\"poc22-$1\",\"duration\":\"1h\",$2}" | jq -r .key)
  [ -n "$k" ] && [ "$k" != null ] || { echo "FAIL no key for $1" >&2; kill $$; }
  printf %s "$k"
}
mcp() {
  printf 'header = "x-litellm-api-key: Bearer %s"\n' "$1" | curl -s -K - "$P/mcp" -H 'content-type: application/json' \
    -H 'accept: application/json, text/event-stream' \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$2\",\"params\":$3}" | sed -n 's/^data: //p'
}
tools() { mcp "$1" tools/list '{}' | jq -c '[.result.tools[].name]'; }
ADMIN='"user_id":"default_user_id","models":["claude-stand-in"]'

echo "== tools per key (keys of the admin user)"
L=$(key list "$ADMIN,\"object_permission\":{\"mcp_servers\":[\"notes\"],\"mcp_tool_permissions\":{\"notes\":[\"read_note\"]}}")
N=$(key none "$ADMIN,\"object_permission\":{\"mcp_servers\":[\"no-mcp-servers\"]}")
B=$(key bare "$ADMIN")
is "key with a tool list: lists that tool only" "$(tools "$L")" '["notes-read_note"]'
has "key with a tool list: the other tool is refused" \
  "$(mcp "$L" tools/call '{"name":"notes-write_note","arguments":{"text":"x"}}')" "not allowed"
is "key with no-mcp-servers: no tool" "$(tools "$N")" '[]'
echo "NOTE key of the admin user without any list: $(tools "$B") (every tool; zero must always send a list)"

echo "== skills per key"
mkdir -p "$W/src/$SKILL"
cat > "$W/src/$SKILL/SKILL.md" <<EOF
---
name: $SKILL
description: Use when asked to run the pong test of zero issue 172. Answers with a fixed token.
---
When asked to run the pong test of zero issue 172, answer with exactly: PONG-172
EOF
(cd "$W/src" && zip -qr "$W/skill.zip" "$SKILL")
is "skill uploaded as a ZIP by the admin" \
  "$(adm -o "$W/up.json" -w '%{http_code}' -F "file=@$W/skill.zip" -F version=1.0.0 "$P/skill-hub/skills/upload")" 200
is "upload answer names the skill" "$(jq -r .name "$W/up.json")" "$SKILL"
is "skill made private (unpublish)" "$(adm -o /dev/null -w '%{http_code}' -X POST "$P/skill-hub/skills/$SKILL/unpublish")" 200
is "admin sees it as private" "$(adm "$P/skill-hub/skills?limit=100" | jq -r ".skills[]|select(.name==\"$SKILL\").visibility")" private
KW=$(key with "\"models\":[\"claude-stand-in\"],\"object_permission\":{\"skills\":[\"$SKILL\"]}")
KO=$(key without "\"models\":[\"claude-stand-in\"]")
is "key with the grant: lists the skill" "$(as "$KW" "$P/skill-hub/skills?limit=100" | jq -c '[.skills[].name]')" "[\"$SKILL\"]"
is "key with the grant: archive" "$(as "$KW" -o "$W/got.zip" -w '%{http_code}' "$P/skill-hub/skills/$SKILL/archive")" 200
mkdir "$W/got" && unzip -q "$W/got.zip" -d "$W/got" || true
has "archive holds SKILL.md at its root" "$(cat "$W/got/SKILL.md" 2>&1)" "PONG-172"
is "key without the grant: skill not listed" "$(as "$KO" "$P/skill-hub/skills?limit=100" | jq -c '[.skills[].name]')" '[]'
is "key without the grant: archive" "$(as "$KO" -o /dev/null -w '%{http_code}' "$P/skill-hub/skills/$SKILL/archive")" 404
is "key without the grant: content" "$(as "$KO" -o /dev/null -w '%{http_code}' "$P/skill-hub/skills/$SKILL/content")" 404

echo "== the fetched skill in a sandbox ($SANDBOX), used by opencode"
sbx 'rm -rf /tmp/poc22; mkdir -p /tmp/poc22/skills/'"$SKILL"' /tmp/poc22/work' >/dev/null
$SSH "cat > /tmp/poc22/skills/$SKILL/SKILL.md" < "$W/got/SKILL.md"
sbx 'printf %s '"'"'{"$schema":"https://opencode.ai/config.json","skills":["/tmp/poc22/skills"]}'"'"' > /tmp/poc22/work/opencode.json' >/dev/null
out=$(sbx 'cd /tmp/poc22/work && KEY=$(python3 -c '"'"'import json,os;print(json.load(open(os.path.expanduser("~/.config/opencode/service.json")))["env"]["LITELLM_API_KEY"])'"'"' 2>/dev/null) && LITELLM_API_KEY=$KEY opencode --standalone run -m litellm/'"$MODEL"' "run the pong test of zero issue 172"')
has "opencode in the sandbox uses the skill" "$out" "PONG-172"

echo "fails: $fails"
[ $fails -eq 0 ]
