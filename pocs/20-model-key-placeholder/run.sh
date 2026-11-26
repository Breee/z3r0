#!/bin/sh
# PoC 20 (#170): the LiteLLM key of a sandbox is held by OpenShell; the sandbox
# only has a placeholder, which OpenShell swaps on the way to the model proxy.
# Needs: a running sandbox of a workspace that uses the model proxy (zero up),
# the proxy on 127.0.0.1:18096, Copilot connected in the proxy.
# Changes the sandbox's rule "models" for the run and puts it back at the end.
set -eu
cd "$(dirname "$0")"
SANDBOX=${SANDBOX:-zero-developer}
MODEL=${MODEL:-github_copilot/claude-haiku-4.5}
PROJECT=${PROJECT:-88739432}
O=${OPENSHELL:-/usr/local/lib/zero/openshell-0.1.8/openshell}
STATE=${XDG_STATE_HOME:-$HOME/.local/state}/zero
P=http://127.0.0.1:18096
U=http://host.openshell.internal:18096
W=$(mktemp -d)
MASTER=$(cat "$STATE/parts/models.key")
fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
hasnot() { if printf %s "$2" | grep -q -- "$3"; then fail "$1" "$2"; else pass "$1"; fi; }
sbx() { ssh -F "$STATE/$SANDBOX/ssh.conf" "openshell-$SANDBOX.default" "$1" </dev/null 2>&1; }
api() { curl -s -X POST "$P$1" -H "Authorization: Bearer $MASTER" -H 'content-type: application/json' -d "$2"; }

cleanup() {
  [ -f "$W/base.yaml" ] && $O policy set "$SANDBOX" --policy "$W/base.yaml" --wait >/dev/null 2>&1 || true
  $O sandbox provider detach --wait "$SANDBOX" poc20-key >/dev/null 2>&1 || true
  $O provider delete poc20-key >/dev/null 2>&1 || true
  $O profile delete poc20-model-key >/dev/null 2>&1 || true
  api /key/delete '{"key_aliases":["poc20"]}' >/dev/null 2>&1 || true
  sbx 'rm -rf /tmp/poc20' >/dev/null || true
  rm -rf "$W"
}
trap cleanup EXIT

api /key/delete '{"key_aliases":["poc20"]}' >/dev/null
KEY=$(api /key/generate '{"key_alias":"poc20","user_id":"default_user_id","models":["github_copilot/*"],"duration":"1h","object_permission":{"mcp_servers":["no-mcp-servers"]}}' | jq -r .key)
[ -n "$KEY" ] && [ "$KEY" != null ] || { echo "FAIL no key from the proxy"; exit 1; }

$O profile import -f profile.yaml >"$W/log" 2>&1 || { cat "$W/log"; exit 1; }
POC20_MODEL_KEY=$KEY $O provider create --name poc20-key --type poc20-model-key --credential POC20_MODEL_KEY >"$W/log" 2>&1 || { cat "$W/log"; exit 1; }
# Attach first, without waiting: until a rule binds the credential to a host the
# sandbox withholds it (status "withheld"). The rule can only name an attached provider.
$O sandbox provider attach "$SANDBOX" poc20-key >"$W/log" 2>&1 || { cat "$W/log"; exit 1; }

$O policy get "$SANDBOX" --base | sed '1,/^---$/d' >"$W/base.yaml"
grep -q 'port: 18096' "$W/base.yaml" || { echo "FAIL $SANDBOX has no rule for the model proxy"; exit 1; }
sed 's/^\( *\)port: 18096$/&\n\1credential_binding:\n\1  provider: poc20-key/' "$W/base.yaml" >"$W/bound.yaml"
$O policy set "$SANDBOX" --policy "$W/bound.yaml" --wait >"$W/log" 2>&1 || { cat "$W/log"; exit 1; }
$O sandbox provider status --wait "$SANDBOX" poc20-key >"$W/log" 2>&1 || { cat "$W/log"; exit 1; }

echo "== what the sandbox holds"
v=$(sbx 'printf %s "$POC20_MODEL_KEY" | cut -c1-17')
has "the sandbox has a placeholder in its environment" "$v" '^openshell:resolve'
hasnot "the placeholder is not a LiteLLM key" "$v" '^sk-'

echo "== curl"
has "model list with the placeholder as Bearer" "$(sbx "curl -s $U/v1/models -H \"Authorization: Bearer \$POC20_MODEL_KEY\"")" "$MODEL"
r=$(sbx "curl -s $U/v1/chat/completions -H \"Authorization: Bearer \$POC20_MODEL_KEY\" -H 'content-type: application/json' -d '{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"answer with the single word: pong\"}]}'")
has "chat completion answered" "$r" 'pong'
r=$(sbx "curl -s $U/v1/messages -H \"x-api-key: \$POC20_MODEL_KEY\" -H 'anthropic-version: 2023-06-01' -H 'content-type: application/json' -d '{\"model\":\"$MODEL\",\"max_tokens\":20,\"messages\":[{\"role\":\"user\",\"content\":\"answer with the single word: pong\"}]}'")
if printf %s "$r" | grep -q pong; then echo "NOTE placeholder in the x-api-key header: answered"; else echo "NOTE placeholder in the x-api-key header: not answered: $(printf %s "$r" | cut -c1-200)"; fi
has "no key: refused by the proxy" "$(sbx "curl -s -o /dev/null -w %{http_code} $U/v1/models")" '401'

echo "== Copilot CLI"
r=$(sbx "mkdir -p /tmp/poc20 && cd /tmp/poc20 && HOME=/tmp/poc20 COPILOT_AUTO_UPDATE=false COPILOT_OFFLINE=true COPILOT_PROVIDER_BASE_URL=$U/v1 COPILOT_PROVIDER_API_KEY=\"\$POC20_MODEL_KEY\" /usr/local/bin/copilot --no-auto-update --model $MODEL -p 'answer with the single word: pong'")
has "Copilot CLI answers with the placeholder as its key" "$r" 'pong'

echo "== opencode"
r=$(sbx "cd /tmp/poc20 && LITELLM_API_KEY=\"\$POC20_MODEL_KEY\" opencode --standalone run -m litellm/$MODEL 'answer with the single word: pong'")
has "opencode answers with the placeholder as its key" "$r" 'pong'

echo "== the placeholder elsewhere"
c=$(sbx "curl -s -o /dev/null -w %{http_code} https://gitlab.com/api/v4/projects/$PROJECT -H \"Authorization: Bearer \$POC20_MODEL_KEY\"")
has "placeholder sent to the forge: refused by OpenShell" "$c" '403'
c=$(sbx "curl -s -o /dev/null -w %{http_code} https://gitlab.com/api/v4/projects/$PROJECT -H \"Authorization: Bearer \$GITLAB_TOKEN\"")
has "control: the forge placeholder still works there" "$c" '200'

echo "== after detach"
$O policy set "$SANDBOX" --policy "$W/base.yaml" --wait >/dev/null 2>&1
$O sandbox provider detach --wait "$SANDBOX" poc20-key >/dev/null 2>&1
c=$(sbx "curl -s -o /dev/null -w %{http_code} $U/v1/models -H \"Authorization: Bearer \$POC20_MODEL_KEY\"")
hasnot "provider detached: the placeholder gets no model list" "$c" '200'

echo "fails: $fails"
[ $fails -eq 0 ]
