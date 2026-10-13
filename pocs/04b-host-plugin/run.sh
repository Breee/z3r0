#!/bin/sh
# PoC 04b: OpenCode on the host, with a plugin that routes the `bash` tool
# into a sandbox. The `read` tool still reads host files (known gap).
#
# Usage: pocs/04b-host-plugin/run.sh [model]
set -u
. "$(dirname "$0")/../lib.sh"
cd "$(dirname "$0")"

SANDBOX=poc04b
MODEL="${1:-$DEFAULT_MODEL}"
trap 'delete_sandbox; rm -f host-marker.txt' EXIT

step "Setup"
echo HOST-ONLY-MARKER > host-marker.txt
openshell sandbox create --name "$SANDBOX" \
  --from docker.io/library/golang:1.25 --detach >/dev/null 2>&1 || exit 1

step "Agent runs on the host"
out="$(Z3R0_SANDBOX=$SANDBOX timeout 300 opencode run -m "$MODEL" \
  'Use the bash tool to run: hostname. Then use the read tool on host-marker.txt. Report both outputs verbatim.' 2>&1)"
echo "$out" | tail -5

step "Checks"
expect bash-routed-to-sandbox contains "$out" "sandbox-$SANDBOX"

if contains "$out" HOST-ONLY-MARKER; then gap  read-tool-runs-on-host
else                                      info read-tool-did-not-read-host
fi

exit $RC
