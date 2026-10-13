#!/bin/sh
# PoC 05: VS Code Remote-SSH into an OpenShell sandbox.
# Copilot Chat runs in the remote extension host, inside the sandbox.
#
# Leaves the sandbox running. Delete it with: scripts/sandbox-down.sh NAME
#
# Usage: pocs/05-vscode-remote/run.sh [sandbox-name]
set -u
. "$(dirname "$0")/../lib.sh"
cd "$ROOT"

SANDBOX="${1:-vsc05}"
SSH_CONFIG="$HOME/.ssh/openshell-$SANDBOX.conf"

# Copilot endpoints, reachable only by binaries under the VS Code server dir
VSCODE_BINARIES='/sandbox/.vscode-server/**'
COPILOT_HOSTS='*.githubcopilot.com
api.enterprise.githubcopilot.com
api.github.com
copilot-proxy.githubusercontent.com'

step "Setup"
POLICY=pocs/05-vscode-remote/policy.yaml \
  scripts/sandbox-up.sh "$SANDBOX" tests/fixtures/hello-go >/dev/null || exit 1
openshell sandbox upload "$SANDBOX" tests/containment /sandbox >/dev/null
git_init_in_sandbox /sandbox/hello-go
in_sandbox 'echo "export PATH=/usr/local/go/bin:\$PATH" >> /sandbox/.profile'  # C09

for host in $COPILOT_HOSTS; do
  openshell policy update "$SANDBOX" \
    --rule-name copilot \
    --binary "$VSCODE_BINARIES" \
    --add-endpoint "$host:443:read-write:rest:enforce" \
    --wait >/dev/null
done

openshell sandbox ssh-config "$SANDBOX" > "$SSH_CONFIG"
SSH_HOST="$(awk '/^Host /{print $2; exit}' "$SSH_CONFIG")"
remote() { ssh -F "$SSH_CONFIG" "$SSH_HOST" "$1"; }

step "Checks"
expect     ssh-non-root           [ "$(remote 'id -u')" != 0 ]
expect_not curl-to-copilot-denied remote 'curl -s -m 5 https://api.githubcopilot.com >/dev/null'
expect_not server-download-denied remote 'curl -s -m 5 https://update.code.visualstudio.com >/dev/null'
expect     pty-works              contains "$(remote 'python3 -c "import pty; pid, fd = pty.fork(); print(1) if pid else None"')" 1

echo
echo "Host:   $SSH_HOST"
echo "Config: $SSH_CONFIG"
exit $RC
