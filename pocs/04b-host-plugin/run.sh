#!/bin/sh
# PoC 04b: host OpenCode + plugin that routes the `bash` tool into a sandbox.
# The `read` tool still reads host files (known gap).
# Usage: pocs/04b-host-plugin/run.sh [model]
set -u
cd "$(dirname "$0")"

NAME=poc04b
MODEL="${1:-opencode/nemotron-3.5-lightning-free}"
rc=0

trap 'openshell sandbox delete "$NAME" >/dev/null 2>&1; rm -f host-marker.txt' EXIT

echo HOST-ONLY-MARKER > host-marker.txt
openshell sandbox create --name "$NAME" --from docker.io/library/golang:1.25 --detach >/dev/null 2>&1 || exit 1

out=$(Z3R0_SANDBOX=$NAME timeout 300 opencode run -m "$MODEL" \
  'Use the bash tool to run: hostname. Then use the read tool on host-marker.txt. Report both outputs verbatim.' 2>&1)
echo "$out" | tail -5

if echo "$out" | grep -q "sandbox-$NAME"; then
  echo "PASS bash routed to sandbox"
else
  echo "FAIL bash routed to sandbox"
  rc=1
fi

if echo "$out" | grep -q HOST-ONLY-MARKER; then
  echo "GAP  read tool runs on host"
else
  echo "INFO read tool did not read host"
fi

exit $rc
