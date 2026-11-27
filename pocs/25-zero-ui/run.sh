#!/bin/sh
# PoC 25 (#163): LiteLLM's web UI below a sub-path, behind one loopback port that stands in for zero-cp's UI listener.
# Needs: PoC 17's state (~/.local/state/zero-poc17: key, pg), go, curl, a compose command (default: docker compose, ADR 0051).
# SANDBOX (default zero-developer) is used for one check: the UI port is not reachable from a sandbox. KEEP=1 leaves it running.
set -eu
cd "$(dirname "$0")"
COMPOSE=${COMPOSE:-docker compose}
SANDBOX=${SANDBOX:-zero-developer}
STATE=${XDG_STATE_HOME:-$HOME/.local/state}/zero-poc17
[ -f "$STATE/key" ] && [ -f "$STATE/pg" ] || { echo "FAIL no state of PoC 17: run pocs/17-litellm/run.sh with UI=1 once"; exit 1; }
export LITELLM_MASTER_KEY="$(cat "$STATE/key")" COPILOT_DIR="$STATE/copilot" PG_PASSWORD="$(cat "$STATE/pg")"
C="$COMPOSE -f ../17-litellm/compose.yaml -f ../17-litellm/compose.ui.yaml -f compose.yaml"
FRONT=127.0.0.1:18111
U=http://$FRONT
W=$(mktemp -d)
FPID=
cleanup() {
  [ -n "$FPID" ] && [ -z "${KEEP:-}" ] && kill "$FPID" 2>/dev/null
  [ -n "${KEEP:-}" ] || $C down >/dev/null 2>&1 || echo "CLEANUP FAILED: compose down"
  rm -rf "$W"
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
is() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "got $2, want $3"; fi; }
code() { curl -s -o /dev/null -m 10 -w '%{http_code}' "$@" || true; }

$C up -d > "$W/up.log" 2>&1 || { cat "$W/up.log"; echo "FAIL compose up"; exit 1; }
i=0; until [ "$(code http://127.0.0.1:4000/litellm/health/liveliness)" = 200 ]; do
  i=$((i+1)); [ $i -gt 150 ] && { echo "FAIL proxy not up after 300 s"; exit 1; }; sleep 2
done
go build -o "$W/front" ./front
"$W/front" "$FRONT" http://127.0.0.1:4000 > "$W/front.log" 2>&1 &
FPID=$!
sleep 1

echo "== LiteLLM below /litellm"
is "LiteLLM answers below the sub-path" "$(code http://127.0.0.1:4000/litellm/health/liveliness)" 200
echo "NOTE LiteLLM at the old place (no sub-path): http $(code http://127.0.0.1:4000/health/liveliness)"

echo "== through the front port"
is "health through the front port" "$(code $U/litellm/health/liveliness)" 200
is "UI page through the front port" "$(code -L $U/litellm/ui/)" 200
curl -s -L -m 10 "$U/litellm/ui/" > "$W/ui.html"
grep -oE '(src|href)="[^"]+\.(js|css)[^"]*"' "$W/ui.html" | sed -E 's/^(src|href)="//; s/"$//' | sort -u > "$W/assets"
n=$(wc -l < "$W/assets"); bad=0; out=0
while read -r a; do
  case "$a" in /litellm/*) ;; *) out=$((out+1));; esac
  [ "$(code "$U$a")" = 200 ] || bad=$((bad+1))
done < "$W/assets"
echo "assets: $n"
[ "$n" -gt 0 ] && pass "the page names script and style files" || fail "the page names script and style files" "none"
is "every file of the page is below the sub-path" "$out" 0
is "every file of the page loads through the front port" "$bad" 0
is "a path outside the sub-path is not passed on" "$(code $U/health/liveliness)" 404
is "another Host name is refused" "$(code -H 'Host: evil.example' $U/litellm/ui/)" 403
is "model list without a key is refused" "$(code $U/litellm/v1/models)" 401

echo "== from a sandbox"
SSH="ssh -F $HOME/.local/state/zero/$SANDBOX/ssh.conf openshell-$SANDBOX.default"
r=$($SSH "echo c=\$(curl -s -o /dev/null -m 8 -w '%{http_code}' http://host.openshell.internal:18111/litellm/ui/)" </dev/null 2>&1 | tail -n 1)
is "the front port is not reachable from the sandbox" "$r" "c=000"

echo "fails: $fails"
[ $fails -eq 0 ]
