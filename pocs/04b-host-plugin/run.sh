#!/bin/sh
# PoC04b: host OpenCode + plugin overriding `bash` -> runs in sandbox; `read` still reads host files (gap).
set -u
cd "$(dirname "$0")"; NAME=poc04b; rc=0
trap 'openshell sandbox delete $NAME >/dev/null 2>&1; rm -f host-marker.txt' EXIT
echo HOST-ONLY-MARKER > host-marker.txt
openshell sandbox create --name $NAME --from docker.io/library/golang:1.25 --detach >/dev/null 2>&1 || exit 1
out="$(Z3R0_SANDBOX=$NAME timeout 300 opencode run -m "${1:-opencode/nemotron-3.5-lightning-free}" \
  'Use the bash tool to run: hostname. Then use the read tool on host-marker.txt. Report both outputs verbatim.' 2>&1)"
echo "$out" | tail -5
echo "$out" | grep -q "sandbox-$NAME" && echo "PASS bash-routed-to-sandbox" || { echo "FAIL bash-routed-to-sandbox"; rc=1; }
echo "$out" | grep -q HOST-ONLY-MARKER && echo "GAP read-tool-runs-on-host" || echo "INFO read-tool-did-not-read-host"
exit $rc
