#!/bin/sh
# PoC 05: VS Code Remote-SSH into an OpenShell sandbox; Copilot Chat runs in the remote extension host.
# Usage: pocs/05-vscode-remote/run.sh [name]   (leaves sandbox running; delete with scripts/sandbox-down.sh NAME)
set -eu
N=${1:-vsc05}
cd "$(dirname "$0")/../.."
pass() { echo "PASS $1"; }; fail() { echo "FAIL $1"; rc=1; }; rc=0

scripts/sandbox-up.sh "$N" tests/fixtures/hello-go >/dev/null
openshell sandbox upload "$N" tests/containment /sandbox >/dev/null

# Copilot endpoints, only for binaries under the VS Code server dir.
B='/sandbox/.vscode-server/**'
for h in '*.githubcopilot.com' api.github.com copilot-proxy.githubusercontent.com; do
  openshell policy update "$N" --rule-name copilot --binary "$B" --add-endpoint "$h:443:read-write:rest:enforce" --wait >/dev/null
done

CFG=$HOME/.ssh/openshell-$N.conf
openshell sandbox ssh-config "$N" > "$CFG"
H=$(awk '/^Host /{print $2;exit}' "$CFG")

[ "$(ssh -F "$CFG" "$H" id -u)" != 0 ] && pass ssh-non-root || fail ssh-non-root
ssh -F "$CFG" "$H" 'curl -s -m 5 https://api.githubcopilot.com >/dev/null' && fail curl-to-copilot-denied || pass curl-to-copilot-denied
ssh -F "$CFG" "$H" 'curl -s -m 5 https://update.code.visualstudio.com >/dev/null' && fail server-download-denied || pass server-download-denied

echo "HOST $H  CONFIG $CFG"
exit $rc
