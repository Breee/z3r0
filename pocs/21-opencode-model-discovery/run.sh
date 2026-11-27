#!/bin/sh
# PoC 21 (#171): how opencode in a zero sandbox gets the model list of the model proxy (LiteLLM).
# Needs: the model proxy of zero running on 127.0.0.1:18096 with a connected provider, and a
# running sandbox of a workspace that uses it (its LiteLLM key is in the opencode service env).
# Works only in /tmp/poc21 inside the sandbox, with project config files and `opencode --standalone`;
# the opencode config zero wrote and the running opencode service are not touched.
# SANDBOX=zero-developer ./run.sh
set -eu
cd "$(dirname "$0")"

SANDBOX=${SANDBOX:-zero-developer}
A=${MODEL_A:-github_copilot/claude-haiku-4.5}
B=${MODEL_B:-github_copilot/gpt-5.4}
C=${MODEL_C:-github_copilot/claude-sonnet-5.5}
CONF=${XDG_STATE_HOME:-$HOME/.local/state}/zero/$SANDBOX/ssh.conf
URL=http://host.openshell.internal:18096/v1

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
hasnot() { if [ -z "$2" ]; then fail "$1" "(no output)"; elif printf %s "$2" | grep -q -- "$3"; then fail "$1" "$2"; else pass "$1"; fi; }
sbx() { ssh -F "$CONF" "openshell-$SANDBOX.default" "$1" </dev/null 2>&1 || true; }

# The key never leaves the sandbox: it is read there, into a variable of the remote shell.
KEY='KEY=$(python3 -c '\''import json,os;print(json.load(open(os.path.expanduser("~/.config/opencode/service.json")))["env"]["LITELLM_API_KEY"])'\'')'
oc() { sbx "$KEY; cd /tmp/poc21/$1 && LITELLM_API_KEY=\$KEY opencode --standalone $2"; }

cleanup() { sbx 'rm -rf /tmp/poc21' >/dev/null; }
trap cleanup EXIT INT TERM HUP

sbx 'rm -rf /tmp/poc21; mkdir -p /tmp/poc21/builtin /tmp/poc21/custom' >/dev/null
ssh -F "$CONF" "openshell-$SANDBOX.default" 'cat > /tmp/poc21/render.py' < render.py
sbx "$KEY; curl -s -m 30 -H \"Authorization: Bearer \$KEY\" $URL/models -o /tmp/poc21/models.json" >/dev/null
echo "opencode: $(sbx 'opencode --version' | tail -1)"

echo "== what the proxy reports"
st=$(sbx 'python3 /tmp/poc21/render.py stats')
echo "  $st"
has "the proxy lists models" "$st" "^models=[1-9]"
has "models carry input and output token limits" "$st" "with_limits=[1-9]"
echo "  without limits: $(sbx 'python3 /tmp/poc21/render.py nolimits')"
hasnot "no model is owned by vllm (the owner opencode's vLLM discovery keeps)" "$st" "owned_by=.*vllm"
for p in /api/v1/models /api/tags; do
  has "the proxy has no $p (LM Studio / Ollama discovery path)" \
    "$(sbx "curl -s -o /dev/null -m 10 -w '%{http_code}' http://host.openshell.internal:18096$p")" "^404$"
done

echo "== built-in runtimes with discovery, pointed at the proxy"
sbx "cat > /tmp/poc21/builtin/opencode.json <<'EOF'
{\"\$schema\": \"https://opencode.ai/config.json\", \"providers\": {
  \"vllm\": {\"settings\": {\"baseURL\": \"$URL\", \"apiKey\": \"{env:LITELLM_API_KEY}\"}},
  \"ollama\": {\"settings\": {\"baseURL\": \"$URL\", \"apiKey\": \"{env:LITELLM_API_KEY}\"}},
  \"lmstudio\": {\"settings\": {\"baseURL\": \"$URL\", \"apiKey\": \"{env:LITELLM_API_KEY}\"}}}}
EOF" >/dev/null
l=$(oc builtin models)
has "opencode lists models at all (control)" "$l" "^litellm/github_copilot/\|^opencode/"
for p in vllm ollama lmstudio; do
  hasnot "$p: lists no model of the proxy" "$l" "^$p/github_copilot"
done

echo "== custom provider with a list written from the proxy's answer"
has "list written (2 models)" "$(sbx "python3 /tmp/poc21/render.py write /tmp/poc21/custom $A $B")" "wrote 2 models"
has "the file carries the limits of the proxy" "$(sbx 'cat /tmp/poc21/custom/opencode.json')" '"context": [1-9]'
l=$(oc custom models)
has "opencode lists the first model" "$l" "^poc21/$A\$"
has "opencode lists the second model" "$l" "^poc21/$B\$"
hasnot "a model not in the file is not listed" "$l" "^poc21/$C\$"
has "prompt answered through the proxy" \
  "$(oc custom "run -m poc21/$A 'what is 17 plus 25? answer with the number only'")" "^42"
has "list change (3 models)" "$(sbx "python3 /tmp/poc21/render.py write /tmp/poc21/custom $A $B $C")" "wrote 3 models"
has "the added model is listed at the next start" "$(oc custom models)" "^poc21/$C\$"

echo "fails: $fails"
[ $fails -eq 0 ]
