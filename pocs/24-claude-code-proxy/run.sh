#!/bin/sh
# PoC 24 (#166): Claude Code in a zero sandbox against the model proxy, with the
# proxy key as a placeholder and no Anthropic login.
# Needs: a sandbox of a workspace with models (zero up), the model proxy running,
# podman (to take the Claude Code binary out of its image) unless CLAUDE names it.
# Changes the sandbox policy for the run (one more program in the models rule) and
# puts it back; works only in /tmp/poc24 inside the sandbox.
set -eu
cd "$(dirname "$0")"
SANDBOX=${SANDBOX:-zero-developer}
MODEL=${MODEL:-github_copilot/claude-haiku-4.5}
IMAGE=ghcr.io/uniget-org/tools/claude:2.1.332@sha256:20749e1942b0312a77dfa0f964f8eab08612ca12a1f42763ffb92e37a7c1aac7
O=${OPENSHELL:-/usr/local/lib/zero/openshell-0.1.8/openshell}
CONF=${XDG_STATE_HOME:-$HOME/.local/state}/zero/$SANDBOX/ssh.conf
W=$(mktemp -d)
changed=0
fails=0

sbx() { ssh -F "$CONF" "openshell-$SANDBOX.default" "$1" </dev/null 2>&1; }
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
is() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "got '$2', want '$3'"; fi; }

cleanup() {
	if [ $changed = 1 ]; then
		"$O" policy set "$SANDBOX" --policy "$W/base.yaml" --wait >/dev/null 2>&1 || echo "CLEANUP FAILED: policy of $SANDBOX not put back ($W/base.yaml kept)"
	fi
	sbx 'rm -rf /tmp/poc24' >/dev/null || echo "CLEANUP FAILED: /tmp/poc24 in the sandbox"
	[ -n "${CID:-}" ] && podman rm "$CID" >/dev/null 2>&1
	[ $changed = 1 ] && [ -f "$W/keep" ] || rm -rf "$W"
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

BIN=${CLAUDE:-}
if [ -z "$BIN" ]; then
	CID=$(podman create "$IMAGE" /bin/true)
	podman cp "$CID:/libexec/claude/node_modules/@anthropic-ai/claude-code-linux-x64/claude" "$W/claude"
	BIN=$W/claude
fi
ssh -F "$CONF" "openshell-$SANDBOX.default" 'mkdir -p /tmp/poc24/home /tmp/poc24/work && cat > /tmp/poc24/claude && chmod +x /tmp/poc24/claude' < "$BIN"
sbx 'echo MARKER-24-OK > /tmp/poc24/work/note.txt' >/dev/null
echo "claude: $(sbx '/tmp/poc24/claude --version')"

# One Claude Code run in the sandbox; $1 = arguments as shell text.
cc() {
	sbx "cd /tmp/poc24/work && HOME=/tmp/poc24/home ANTHROPIC_BASE_URL=http://host.openshell.internal:18096 ANTHROPIC_AUTH_TOKEN=\"\$LITELLM_API_KEY\" ANTHROPIC_MODEL=$MODEL ANTHROPIC_DEFAULT_HAIKU_MODEL=$MODEL CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 DISABLE_AUTOUPDATER=1 DISABLE_TELEMETRY=1 DISABLE_ERROR_REPORTING=1 timeout 120 /tmp/poc24/claude $1"
}
ASK="-p 'what is 17 plus 25? answer with the number only'"

echo "== without a rule for the program"
r=$(cc "$ASK" || true)
if printf %s "$r" | grep -qx 42; then fail "no answer without a rule" "$r"; else pass "no answer without a rule for the program"; fi

echo "== with the program in the models rule"
"$O" policy get "$SANDBOX" --base | sed '1,/^---$/d' > "$W/base.yaml"
awk '/^  [a-z0-9_-]+:$/ { m = ($1 == "models:") } { print } m && /- path: \/usr\/bin\/curl$/ { print "      - path: /tmp/poc24/claude" }' "$W/base.yaml" > "$W/poc.yaml"
is "program added to the models rule only" "$(grep -c '/tmp/poc24/claude' "$W/poc.yaml")" 1
touch "$W/keep"; changed=1
"$O" policy set "$SANDBOX" --policy "$W/poc.yaml" --wait >/dev/null
is "prompt answered through the proxy" "$(cc "$ASK" | tail -n 1)" 42
has "tool use: reads a file" "$(cc "--allowedTools Read -p 'read the file note.txt and print its content exactly'")" MARKER-24-OK
has "tool use: runs a shell command" "$(cc "--permission-mode bypassPermissions -p 'run the shell command: echo SHELL-24-OK, and print its output'")" SHELL-24-OK

echo "== what the sandbox holds and reaches"
has "the proxy key in the sandbox is a placeholder" "$(sbx 'printf %s "$LITELLM_API_KEY" | cut -c1-17')" '^openshell:resolve'
is "no key in what Claude Code wrote" "$(sbx 'grep -rlE "(^|[^A-Za-z0-9])sk-[A-Za-z0-9_-]{16,}" /tmp/poc24/home 2>/dev/null | wc -l')" 0
is "api.anthropic.com not reachable" "$(sbx 'curl -s -o /dev/null -m 10 -w "%{http_code}" https://api.anthropic.com/' | tail -n 1)" 000
is "no Anthropic login in the sandbox" "$(sbx 'env | grep -c "^ANTHROPIC_" ; true' | tail -n 1)" 0

"$O" policy set "$SANDBOX" --policy "$W/base.yaml" --wait >/dev/null && { changed=0; rm -f "$W/keep"; }
echo "fails: $fails"
[ $fails -eq 0 ]
