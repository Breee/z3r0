#!/bin/sh
# PoC 05: VS Code Remote-SSH into an OpenShell sandbox.
# Copilot Chat runs in the remote extension host, inside the sandbox.
# Leaves the sandbox running; delete with: scripts/sandbox-down.sh NAME
# Usage: pocs/05-vscode-remote/run.sh [name]
set -eu
cd "$(dirname "$0")/../.."

NAME="${1:-vsc05}"
rc=0

pass() { echo "PASS $1"; }
fail() { echo "FAIL $1"; rc=1; }

echo "== Setup"
POLICY=pocs/05-vscode-remote/policy.yaml scripts/sandbox-up.sh "$NAME" tests/fixtures/hello-go >/dev/null
openshell sandbox upload "$NAME" tests/containment /sandbox >/dev/null
openshell sandbox exec -n "$NAME" -- sh -c 'cd /sandbox/hello-go && git init -q && git add -A && git -c user.email=z3r0@local -c user.name=z3r0 commit -qm init'
openshell sandbox exec -n "$NAME" -- sh -c 'echo "export PATH=/usr/local/go/bin:\$PATH" >> /sandbox/.profile'  # C09

# Copilot endpoints, only for binaries under the VS Code server dir
for host in '*.githubcopilot.com' api.enterprise.githubcopilot.com api.github.com copilot-proxy.githubusercontent.com; do
  openshell policy update "$NAME" --rule-name copilot --binary '/sandbox/.vscode-server/**' \
    --add-endpoint "$host:443:read-write:rest:enforce" --wait >/dev/null
done

CONFIG="$HOME/.ssh/openshell-$NAME.conf"
openshell sandbox ssh-config "$NAME" > "$CONFIG"
HOST=$(awk '/^Host /{print $2; exit}' "$CONFIG")

echo "== Checks"
if [ "$(ssh -F "$CONFIG" "$HOST" id -u)" != 0 ]; then pass "ssh non-root"; else fail "ssh non-root"; fi

if ssh -F "$CONFIG" "$HOST" 'curl -s -m 5 https://api.githubcopilot.com >/dev/null'; then
  fail "curl to copilot denied"
else
  pass "curl to copilot denied"
fi

if ssh -F "$CONFIG" "$HOST" 'curl -s -m 5 https://update.code.visualstudio.com >/dev/null'; then
  fail "server download denied"
else
  pass "server download denied"
fi

if ssh -F "$CONFIG" "$HOST" 'python3 -c "import pty; pid, fd = pty.fork(); print(1) if pid else None"' | grep -q 1; then
  pass "pty works"
else
  fail "pty works"
fi

echo "Host:   $HOST"
echo "Config: $CONFIG"
exit $rc
