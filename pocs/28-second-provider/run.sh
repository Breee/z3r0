#!/bin/sh
# PoC 28 (#177): a second model provider behind the model proxy, here a stand-in
# that wants an API key. The key is named in the proxy's config and handed to
# the proxy container through the environment of the compose command only.
# Needs the state of PoC 17 (~/.local/state/zero-poc17: key, pg). KEEP=1 leaves it running.
set -eu
cd "$(dirname "$0")"
COMPOSE=${COMPOSE:-docker compose}
STATE=${XDG_STATE_HOME:-$HOME/.local/state}/zero-poc17
[ -s "$STATE/key" ] && [ -s "$STATE/pg" ] || { echo "FAIL no state of PoC 17; run pocs/17-litellm/run.sh with UI=1 first"; exit 1; }
SECRET="acme-$(head -c 18 /dev/urandom | base64 | tr -d '/+=')"
export LITELLM_MASTER_KEY="$(cat "$STATE/key")" COPILOT_DIR="$STATE/copilot" PG_PASSWORD="$(cat "$STATE/pg")"
export POC28="$PWD" ACME_BACKEND_KEY="$SECRET"
C="$COMPOSE -f ../17-litellm/compose.yaml -f ../17-litellm/compose.ui.yaml -f compose.yaml"
P=http://127.0.0.1:4000
W=$(mktemp -d)

auth() { printf 'header = "Authorization: Bearer %s"\n' "$1"; }
adm() { auth "$LITELLM_MASTER_KEY" | curl -s -K - "$@"; }
api() { adm -H 'content-type: application/json' "$P$1" -d "$2"; }
as() { k=$1; shift; auth "$k" | curl -s -K - "$@"; }
cleanup() {
	api /key/delete '{"key_aliases":["poc28-a","poc28-other"]}' >/dev/null 2>&1 || true
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
up() {
	$C up -d > "$W/up.log" 2>&1 || { cat "$W/up.log"; echo "FAIL compose up"; exit 1; }
	i=0
	until [ "$(curl -s -o /dev/null -w '%{http_code}' "$P/health/liveliness")" = 200 ]; do
		i=$((i+1)); [ $i -lt 150 ] || { echo "FAIL proxy not up"; $C logs --tail 30 litellm; exit 1; }
		sleep 2
	done
}
key() { api /key/generate "{\"key_alias\":\"poc28-$1\",\"duration\":\"1h\",$2}" | jq -r .key; }
chat() { as "$1" -H 'content-type: application/json' "$P/v1/chat/completions" -d "{\"model\":\"$2\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}"; }

echo "== the provider's key in the proxy's environment"
ACME_API_KEY="$SECRET" up
A=$(key a '"models":["acme/*"]')
O=$(key other '"models":["github_copilot/*"]')
has "prompt answered by the second provider" "$(chat "$A" acme/small | jq -r '.choices[0].message.content')" 'acme says 42 as small'
has "another of its models answers too" "$(chat "$A" acme/large | jq -r '.choices[0].message.content')" 'as large'
m=$(as "$A" "$P/v1/models" | jq -c '[.data[].id]')
echo "NOTE model list for the key: $m"
has "a key without the provider in its list is refused" "$(chat "$O" acme/small)" 'error'
is "the provider itself refuses a call without its key" "$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:18101/v1/models)" 401
is "the key's value is in no file of this PoC" "$(grep -rl -- "$SECRET" . ../17-litellm 2>/dev/null | wc -l)" 0
if $C logs litellm 2>&1 | grep -q -- "$SECRET"; then fail "key not in the proxy log" ""; else pass "key not in the proxy log"; fi
if as "$A" "$P/v1/models" | grep -q -- "$SECRET" || chat "$A" acme/small | grep -q -- "$SECRET"; then fail "key in no answer" ""; else pass "key in no answer"; fi

echo "== the key missing from the environment"
ACME_API_KEY= up
r=$(chat "$A" acme/small)
echo "NOTE proxy starts without the key; a call then gives: $(printf %s "$r" | jq -r '.error.message // .choices[0].message.content' | cut -c1-160)"
has "no answer without the key" "$r" 'error'

echo "== the key back"
ACME_API_KEY="$SECRET" up
has "prompt answered again" "$(chat "$A" acme/small | jq -r '.choices[0].message.content')" 'acme says 42'
echo "fails: $fails"
[ $fails -eq 0 ]
