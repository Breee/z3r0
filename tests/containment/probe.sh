#!/bin/sh
# Hostile-agent probes. Runs INSIDE the sandbox. Each case must be BLOCKED.
# Output: "PASS|FAIL <case>" lines; exit 1 if any FAIL.
# Usage: probe.sh <host-home> <canary>
H="$1"; C="$2"; rc=0
blocked() { # <name> <cmd...>: PASS if cmd fails
  n="$1"; shift
  if "$@" >/dev/null 2>&1; then echo "FAIL $n"; rc=1; else echo "PASS $n"; fi
}

allowed() { # control: PASS if cmd succeeds (proves probes can detect access)
  n="$1"; shift
  if "$@" >/dev/null 2>&1; then echo "PASS $n"; else echo "FAIL $n"; rc=1; fi
}
allowed "control:read-workspace" cat /sandbox/containment/probe.sh
command -v python3 >/dev/null || echo "SKIP proc:python (no python3 in image)"

# --- filesystem (step 2) ---
for p in .ssh .kube .config/gh .gitconfig repos; do
  blocked "fs:read-host-$p" ls "$H/$p"
done
blocked "fs:host-home-visible" test -e "$H"
blocked "fs:canary-in-fs" grep -rqsI --exclude-dir=proc --exclude-dir=sys --exclude-dir=go --exclude-dir=containment "$C" /
for p in .ssh .kube .config/gh; do
  blocked "fs:sandbox-home-$p" test -e "/sandbox/$p"
done

# --- child processes repeat the same read (step 2) ---
blocked "proc:sh-c"      sh -c "ls '$H/.ssh'"
blocked "proc:bg"        sh -c "(ls '$H/.ssh' > /tmp/bg.out 2>&1 &); sleep 1; test -s /tmp/bg.out && ! grep -q 'No such' /tmp/bg.out"
blocked "proc:python"    sh -c "command -v python3 && python3 -c 'import os;os.listdir(\"$H/.ssh\")'"
exit $rc
