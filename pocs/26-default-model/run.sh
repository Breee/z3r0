#!/bin/sh
# PoC 26 (#176): a default model for opencode, the Copilot CLI and Claude Code
# in a sandbox that uses the model proxy.
# Needs: a running sandbox of a workspace with models (zero up), the model
# proxy with a connected provider. Works only in /tmp/poc26 in the sandbox;
# zero's own opencode config is copied, not changed.
set -eu
SANDBOX=${SANDBOX:-zero-developer}
A=${MODEL_A:-github_copilot/claude-haiku-4.5}
B=${MODEL_B:-github_copilot/claude-sonnet-5.5}
C="$HOME/.local/state/zero/$SANDBOX/ssh.conf"
H="openshell-$SANDBOX.default"
P=http://host.openshell.internal:18096
ASK='what is 17 plus 25? answer with the number only'

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
hasnot() { if [ -n "$2" ] && ! printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
sbx() { ssh -F "$C" "$H" "$1" </dev/null 2>&1 || true; }
cleanup() { sbx 'rm -rf /tmp/poc26' >/dev/null || echo "CLEANUP FAILED: /tmp/poc26 in the sandbox"; }
trap cleanup EXIT INT TERM HUP

# A home of its own with a copy of the config zero wrote, plus a model line.
out=$(sbx "set -e; rm -rf /tmp/poc26; mkdir -p /tmp/poc26/home/.config/opencode /tmp/poc26/plain /tmp/poc26/repo
python3 - <<PY
import json, os
c = json.load(open(os.path.expanduser('~/.config/opencode/opencode.json')))
c['model'] = 'litellm/$A'
json.dump(c, open('/tmp/poc26/home/.config/opencode/opencode.json', 'w'))
json.dump({'\$schema': 'https://opencode.ai/config.json', 'model': 'litellm/$B'}, open('/tmp/poc26/repo/opencode.json', 'w'))
PY
echo ready")
has "config copy with a model line written" "$out" ready

oc() { sbx "cd /tmp/poc26/$1 && HOME=/tmp/poc26/home XDG_CONFIG_HOME=/tmp/poc26/home/.config timeout 120 opencode --standalone run $2 '$ASK'"; }

echo "== opencode"
o=$(oc plain "")
has "user config model: prompt answered without a model argument" "$o" 42
has "user config model: that model is used" "$o" "${A#*/}"
o=$(oc repo "")
has "repository config: prompt answered" "$o" 42
has "repository config wins over the user config" "$o" "${B#*/}"
hasnot "repository config: user config model not used" "$o" "${A#*/}"
o=$(oc repo "--model litellm/$A")
has "--model wins over both" "$o" "${A#*/}"

echo "== Copilot CLI"
cp() { sbx "cd /tmp/poc26/plain && HOME=/tmp/poc26/home $1 COPILOT_AUTO_UPDATE=false COPILOT_OFFLINE=true COPILOT_PROVIDER_BASE_URL=$P/v1 COPILOT_PROVIDER_API_KEY=\"\$LITELLM_API_KEY\" timeout 120 /usr/local/bin/copilot --no-auto-update -p '$ASK'"; }
has "COPILOT_MODEL in the environment: answered without --model" "$(cp "COPILOT_MODEL=$A")" 42
has "no model at all: refused" "$(cp "")" "require an explicit model"

echo "== Claude Code"
cc() { sbx "cd /tmp/poc26/plain && HOME=/tmp/poc26/home $1 ANTHROPIC_BASE_URL=$P ANTHROPIC_AUTH_TOKEN=\"\$LITELLM_API_KEY\" CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 CLAUDE_CODE_DISABLE_OFFICIAL_MARKETPLACE_AUTOINSTALL=1 DISABLE_AUTOUPDATER=1 DISABLE_UPDATES=1 DISABLE_TELEMETRY=1 DISABLE_ERROR_REPORTING=1 timeout 120 /usr/local/bin/claude -p '$ASK'"; }
o=$(cc "ANTHROPIC_MODEL=$A ANTHROPIC_DEFAULT_HAIKU_MODEL=$A" | tail -n 1)
if [ "$o" = 42 ]; then pass "ANTHROPIC_MODEL in the environment: answered without --model"; else fail "ANTHROPIC_MODEL in the environment" "$o"; fi

echo "fails: $fails"
[ $fails -eq 0 ]
