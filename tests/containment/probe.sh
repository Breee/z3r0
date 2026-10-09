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

# --- privilege (step 3) ---
blocked "priv:uid-0"        test "$(id -u)" = 0
blocked "priv:gid-0"        test "$(id -g)" = 0
blocked "priv:sudo"         sudo -n true
blocked "priv:su"           sh -c "echo | su -c true root"
# setuid bins (su, passwd) exist in image; neutralised by NoNewPrivs=1 + CapBnd=0 (cases below)
blocked "priv:cap-eff"      sh -c "grep '^CapEff' /proc/self/status | grep -vq '0000000000000000'"
blocked "priv:cap-bnd"      sh -c "grep '^CapBnd' /proc/self/status | grep -vq '0000000000000000'"
blocked "priv:no-new-privs-0" sh -c "grep -q '^NoNewPrivs:[[:space:]]*0' /proc/self/status"
blocked "priv:write-etc"    sh -c "echo x > /etc/z3r0-probe"
blocked "priv:write-passwd" sh -c "echo x >> /etc/passwd"
blocked "priv:unshare-user" unshare -r true
blocked "priv:mount"        mount -t tmpfs none /tmp

# --- host container engine (step 4) ---  arg 3 = host uid
U="${3:-1000}"
for s in /run/user/$U/podman/podman.sock /run/podman/podman.sock /var/run/docker.sock /run/docker.sock /run/user/$U/docker.sock; do
  blocked "podman:sock-$s" test -e "$s"
done
blocked "podman:any-sock"   sh -c "find / -xdev -type s 2>/dev/null | grep -Eq 'podman|docker|containerd'"
blocked "podman:cli"        sh -c "command -v podman || command -v docker"
blocked "podman:api-curl"   curl -sf --max-time 3 --unix-socket /run/user/$U/podman/podman.sock http://d/_ping
blocked "podman:api-tcp"    curl -sf --max-time 3 http://host.containers.internal:2375/_ping
blocked "podman:host-storage" ls "$H/.local/share/containers"
blocked "mount:write-supervisor-bin" sh -c "echo x >> /opt/openshell/bin/openshell-sandbox"
blocked "mount:write-channel" sh -c "echo x > /.openshell/channel/z3r0-probe"

# --- egress, default policy (step 5) ---
blocked "net:https-example"  curl -sf --max-time 5 https://example.com
blocked "net:http-ip"        curl -sf --max-time 5 http://1.1.1.1
blocked "net:noproxy"        curl -sf --max-time 5 --noproxy '*' https://example.com
blocked "net:ipv6"           curl -sf --max-time 5 -6 https://example.com
# resolver is synthetic (198.18.0.0/15, even for .invalid) -> no upstream DNS query
blocked "net:dns-real-answer" sh -c "getent hosts example.com | grep -vq '^198\.1[89]\.'"
allowed "net:dns-synthetic"  sh -c "getent hosts x.z3r0test.invalid | grep -q '^198\.1[89]\.'"
blocked "net:tcp-raw"        sh -c "timeout 5 bash -c 'exec 3<>/dev/tcp/1.1.1.1/53'"
blocked "net:host-gateway"   curl -sf --max-time 5 -k https://host.containers.internal:17670
exit $rc
