#!/bin/sh
# Gateway/supervisor attacks. Runs INSIDE sandbox. Supervisor = PID 1, same uid as agent.
rc=0
blocked() { n="$1"; shift; if sh -c "$1" >/dev/null 2>&1; then echo "FAIL $n"; rc=1; else echo "PASS $n"; fi; }
allowed() { n="$1"; shift; if sh -c "$1" >/dev/null 2>&1; then echo "PASS $n"; else echo "FAIL $n"; rc=1; fi; }
tcp() { echo "timeout 4 bash -c 'exec 3<>/dev/tcp/$1/$2'"; }
allowed "control:pid1-is-supervisor" "grep -q openshell-sandbox /proc/1/cmdline"
blocked "sup:kill"            "kill -0 1"
blocked "sup:stop"            "kill -STOP 1"
blocked "sup:read-environ"    "cat /proc/1/environ"
blocked "sup:read-mem"        "head -c1 /proc/1/mem"
blocked "sup:read-fds"        "ls /proc/1/fd"
blocked "sup:read-maps"       "cat /proc/1/maps"
blocked "sup:list-state-dir"  "ls /.openshell"
blocked "sup:ca-dir"          "ls /run/openshell-supervisor-ca"
blocked "sup:ca-private-key"  "grep -rqs --exclude-dir=attacks 'PRIVATE KEY' /run /tmp /etc /sandbox /.openshell"
blocked "sup:write-ca-bundle" "touch /run/openshell-supervisor-ca/material/x"
blocked "sup:other-pids"      "test \$(ls /proc | grep -c '^[0-9]') -gt 8"
blocked "gw:netns-has-routes" "grep -qv '^Iface' /proc/net/route"
blocked "gw:tcp-localhost"    "$(tcp 127.0.0.1 17670)"
blocked "gw:tcp-podman-bridge" "$(tcp 10.88.0.1 17670)"
blocked "gw:tcp-slirp-host"   "$(tcp 10.0.2.2 17670)"
blocked "gw:metadata-169"     "curl -sf -m5 http://169.254.169.254/"
exit $rc
