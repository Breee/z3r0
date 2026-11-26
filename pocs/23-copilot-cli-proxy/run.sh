#!/bin/sh
# PoC 23 (#173): GitHub Copilot CLI in a zero sandbox against the model proxy (LiteLLM), no GitHub login.
# Needs: the model proxy of zero running on 127.0.0.1:18096 with a connected provider, and a
# running sandbox of a workspace that uses it (its LiteLLM key is in the opencode service env).
# Adds one policy rule (copilot-probe) to the sandbox for the run and removes it again.
# SANDBOX=zero-developer MODEL=github_copilot/claude-haiku-4.5 ./run.sh
set -eu
cd "$(dirname "$0")"

SANDBOX=${SANDBOX:-zero-developer}
MODEL=${MODEL:-github_copilot/claude-haiku-4.5}
O=${OPENSHELL:-/usr/local/lib/zero/openshell-0.1.8/openshell}
CONF=${XDG_STATE_HOME:-$HOME/.local/state}/zero/$SANDBOX/ssh.conf
BIN=/usr/local/bin/copilot
PROXY=host.openshell.internal:18096

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
hasnot() { if printf %s "$2" | grep -q -- "$3"; then fail "$1" "$2"; else pass "$1"; fi; }
sbx() { ssh -F "$CONF" "openshell-$SANDBOX.default" "$1" </dev/null 2>&1 || true; }

# The key never leaves the sandbox: it is read there, into a variable of the remote shell.
KEY='KEY=$(python3 -c '\''import json,os;print(json.load(open(os.path.expanduser("~/.config/opencode/service.json")))["env"]["LITELLM_API_KEY"])'\'')'
ENV="HOME=/tmp/poc23 COPILOT_AUTO_UPDATE=false COPILOT_OFFLINE=true COPILOT_PROVIDER_BASE_URL=http://$PROXY/v1 COPILOT_PROVIDER_API_KEY=\$KEY"
cli() { sbx "$KEY; mkdir -p /tmp/poc23/work; cd /tmp/poc23/work; $ENV $BIN --no-auto-update $1"; }

cleanup() {
  "$O" policy update "$SANDBOX" --remove-rule copilot-probe --wait >/dev/null 2>&1 || true
  sbx 'rm -rf /tmp/poc23' >/dev/null
}
trap cleanup EXIT

echo "copilot: $(sbx "$BIN --version" | head -1)"

echo "== without a rule for the binary"
has "proxy not reachable for the Copilot CLI, no answer" "$(cli "--model $MODEL -p 'answer with the single word: pong'")" "rror"

"$O" policy update "$SANDBOX" --rule-name copilot-probe --binary "$BIN" \
  --add-endpoint "$PROXY:read-write:rest:enforce" --wait >/dev/null

echo "== with a rule: the binary may reach the proxy, nothing else"
has "prompt answered through the proxy" "$(cli "--model $MODEL -p 'answer with the single word: pong'")" "pong"
out=$(cli "-p 'answer with the single word: pong'")
hasnot "without --model: no answer" "$out" "^pong"
echo "  without --model it says: $(printf %s "$out" | grep -v '^$' | head -2 | cut -c1-200 | tr '\n' ' ')"

sbx 'mkdir -p /tmp/poc23/work; echo MARKER-23-OK > /tmp/poc23/work/marker.txt' >/dev/null
has "tool use: reads a file and gives its content" \
  "$(cli "--model $MODEL --allow-all-tools -p 'read the file marker.txt in the current folder and answer with its content only'")" "MARKER-23-OK"
has "tool use: runs a shell command" \
  "$(cli "--model $MODEL --allow-all-tools -p 'run the shell command: echo SHELL-\$((20+3))-OK  and answer with its output only'")" "SHELL-23-OK"

echo "== what the sandbox holds and reaches"
has "api.githubcopilot.com not reachable from the sandbox" \
  "$(sbx "curl -s -o /dev/null -m 10 -w '%{http_code}' https://api.githubcopilot.com/models")" "^000$"
has "api.github.com not reachable from the sandbox" \
  "$(sbx "curl -s -o /dev/null -m 10 -w '%{http_code}' https://api.github.com/")" "^000$"
hasnot "no GitHub token in the sandbox environment" "$(sbx 'env')" "gho_\|ghu_\|github_pat_"
has "no Copilot login variable in the sandbox" "$(sbx 'echo "[${COPILOT_TOKEN:-}${COPILOT_GITHUB_TOKEN:-}${GH_TOKEN:-}${GITHUB_TOKEN:-}]"')" "^\[\]$"

echo "fails: $fails"
[ $fails -eq 0 ]
