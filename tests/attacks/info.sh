#!/bin/sh
# Info leaks. Runs INSIDE sandbox. Usage: info.sh <host-user> <host-hostname>
U="$1"; HN="$2"; rc=0
blocked() { n="$1"; shift; if sh -c "$1" >/dev/null 2>&1; then echo "FAIL $n"; rc=1; else echo "PASS $n"; fi; }
gap()     { n="$1"; r="$2"; shift 2; if sh -c "$1" >/dev/null 2>&1; then echo "GAP  $n ($r)"; else echo "PASS $n"; fi; }
blocked "info:sys-net"       "ls /sys/class/net"
blocked "info:sys-dmi"       "cat /sys/class/dmi/id/product_name"
blocked "info:dev-list"      "ls /dev"
blocked "info:cgroup-path"   "grep -qv '^0::/\$' /proc/self/cgroup"
blocked "info:env-host-user" "env | grep -q '$U'"
gap "info:mounts-host-user"  C11 "grep -q '$U' /proc/self/mountinfo"
gap "info:hosts-host-name"   C11 "grep -qw '$HN' /etc/hosts"
gap "info:hosts-host-ip"     C11 "grep -q 'host.containers.internal' /etc/hosts"
gap "info:kernel-version"    C24 "uname -r | grep -q '[0-9]'"
exit $rc
