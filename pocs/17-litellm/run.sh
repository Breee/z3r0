#!/bin/sh
# PoC 17: starts LiteLLM + a stand-in backend with compose and measures what the proxy costs.
# COMPOSE="docker compose" uses Docker. KEEP=1 leaves it running (for the Copilot login).
set -eu
cd "$(dirname "$0")"
COMPOSE=${COMPOSE:-podman compose}
ENGINE=${COMPOSE%% *}
STATE=${XDG_STATE_HOME:-$HOME/.local/state}/zero-poc17
mkdir -p "$STATE/copilot"
[ -f "$STATE/key" ] || (umask 077; printf 'sk-%s' "$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')" > "$STATE/key")
export LITELLM_MASTER_KEY="$(cat "$STATE/key")" COPILOT_DIR="$STATE/copilot"
P=http://127.0.0.1:4000 B=http://127.0.0.1:18101
[ -n "${KEEP:-}" ] || trap '$COMPOSE down >/dev/null 2>&1' EXIT

ms() { echo $(( $(date +%s%N) / 1000000 )); }
t0=$(ms)
$COMPOSE up -d >"$STATE/up.log" 2>&1 || { cat "$STATE/up.log"; echo "FAIL compose up"; exit 1; }
until curl -sf -o /dev/null $P/health/liveliness; do
    [ $(( $(ms) - t0 )) -lt 120000 ] || { cat "$STATE/up.log"; $COMPOSE ps; echo "FAIL proxy not up after 120 s"; exit 1; }
    sleep 0.2
done
echo "start: $(( $(ms) - t0 )) ms until /health/liveliness answers"
C=$($COMPOSE ps -q litellm)
echo "image: $($ENGINE image inspect --format '{{.Size}}' "$($ENGINE inspect --format '{{.Image}}' "$C")") bytes"
stats() { echo "$1: $($ENGINE stats --no-stream --format '{{.MemUsage}} cpu {{.CPUPerc}}' "$C")"; }
sleep 5; stats idle

J='{"model":"claude-stand-in","max_tokens":16,"messages":[{"role":"user","content":"ping"}]}'
post() { curl -s -H "authorization: Bearer $2" -H 'content-type: application/json' -d "$J" "$1"; }
echo "openai form:    $(post $P/v1/chat/completions "$LITELLM_MASTER_KEY" | cut -c1-160)"
echo "anthropic form: $(post $P/v1/messages "$LITELLM_MASTER_KEY" | tr -d "\n" | grep -o "\"text\": *\"[^\"]*\"" | head -1)"
echo "no key:         $(curl -s -o /dev/null -w '%{http_code}' -H 'content-type: application/json' -d "$J" $P/v1/chat/completions)"

loop() { t=$(ms); i=0; while [ $i -lt 50 ]; do post "$1" "$2" >/dev/null; i=$((i+1)); done; echo $(( ($(ms) - t) / 50 )); }
d=$(loop $B/v1/chat/completions none); p=$(loop $P/v1/chat/completions "$LITELLM_MASTER_KEY")
echo "per request: backend alone $d ms, through the proxy $p ms"
stats "after 100 requests"
if $COMPOSE logs litellm 2>&1 | grep -q "$LITELLM_MASTER_KEY"; then echo "FAIL key in the log"; exit 1; fi
echo "log: no key"
