#!/bin/sh
# PoC 30 (#167): the gateway under its own user `openshell` with Docker,
# as zero install would set it up. PoC 29 ran the gateway as the login user.
#   sudo ./run.sh                # rootless: a throwaway Docker daemon of `openshell`
#   sudo MODE=rootful ./run.sh   # the system daemon; gateway process in group `docker`
# Needs root (it starts processes as `openshell`), zero installed (user
# `openshell`, OpenShell 0.1.8, /var/lib/zero/input), Docker with rootless extras.
# Installs nothing; removes everything it made.
set -eu
cd "$(dirname "$0")"

[ "$(id -u)" = 0 ] && [ -n "${SUDO_USER:-}" ] || { echo "run with sudo: sudo $0"; exit 1; }
MODE=${MODE:-rootless}
LU=$SUDO_USER
LH=$(getent passwd "$LU" | cut -d: -f6)
OU=openshell
OUID=$(id -u $OU); OGID=$(id -g $OU)
OH=$(getent passwd $OU | cut -d: -f6)
IMAGE=${IMAGE:-ghcr.io/breee/z3r0-dev@sha256:a6d406ae281bd0065f873b77a3521316f0d687df4cde3465298dc3163dfcf1af}
PORT=${PORT:-17690}
LIB=${LIB:-/usr/local/lib/zero/openshell-0.1.8}
G=$LIB/openshell-gateway
NAME=poc30
D=$(mktemp -d -p "$OH" zero-poc30.XXXXXX)       # gateway + daemon state, on disk
R=/run/user/$OUID/zero-poc30                    # daemon socket
IN=$(mktemp -d -p /var/lib/zero/input poc30.XXXXXX)
REG=$LH/.config/openshell/gateways/$NAME
PID=; DPID=
case $MODE in
rootless) SOCKET=$R/docker.sock; GROUPS_=$OGID;;
rootful) SOCKET=/var/run/docker.sock; GROUPS_=$OGID,$(getent group docker | cut -d: -f3);;
*) echo "MODE is rootless or rootful"; exit 1;;
esac
DOCKER="docker -H unix://$SOCKET"

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
is() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "got $2, want $3"; fi; }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
# a process of `openshell`, with exactly the groups named
asou() { g=$1; shift; setpriv --reuid "$OUID" --regid "$OGID" --groups "$g" env -i PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
	HOME="$OH" XDG_RUNTIME_DIR="/run/user/$OUID" XDG_CONFIG_HOME="$D/cfg" XDG_STATE_HOME="$D/state" \
	XDG_DATA_HOME="$D/share" OPENSHELL_LOCAL_TLS_DIR="$D/tls" OPENSHELL_TELEMETRY_ENABLED=false \
	OPENSHELL_SERVER_PORT="$PORT" OPENSHELL_GATEWAY_CONFIG="$D/cfg/openshell/gateway.toml" \
	DOCKERD_ROOTLESS_ROOTLESSKIT_STATE_DIR="$R/rootlesskit" "$@"; }
aslu() { sudo -u "$LU" -H "$@"; }
O() { aslu openshell --gateway $NAME "$@"; }

cleanup() {
	O sandbox delete $NAME >/dev/null 2>&1 || true
	sleep 5
	[ -n "$PID" ] && kill "$PID" 2>/dev/null || true
	if [ "$MODE" = rootful ]; then
		left=$($DOCKER ps -aq --filter "name=openshell-default--$NAME" | wc -l)
		[ "$left" = 0 ] || echo "CLEANUP FAILED: $left containers of $NAME left at $SOCKET"
	fi
	if [ -n "$DPID" ]; then
		kill "$DPID" 2>/dev/null || true
		i=0; while kill -0 "$DPID" 2>/dev/null && [ $i -lt 30 ]; do sleep 1; i=$((i+1)); done
		kill -0 "$DPID" 2>/dev/null && echo "CLEANUP FAILED: throwaway Docker daemon $DPID still runs"
	fi
	sleep 1
	ss -ltn 2>/dev/null | grep -q ":$PORT " && echo "CLEANUP FAILED: something still listens on port $PORT"
	aslu openshell gateway remove $NAME >/dev/null 2>&1 || true
	aslu openshell gateway select openshell >/dev/null 2>&1 || echo "CLEANUP FAILED: active gateway not put back to openshell"
	rm -rf "$REG" "$D" "$IN" "$R"
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

chown "$OUID:$OGID" "$D"; chmod 700 "$D"
asou "$OGID" mkdir -p "$D/cfg/openshell" "$D/state" "$D/share" "$D/tls" "$D/data" "$R"

if [ "$MODE" = rootless ]; then
	# the daemon zero install would set up as a user unit of `openshell`
	asou "$OGID" dockerd-rootless.sh --host "unix://$SOCKET" --data-root "$D/data" \
		--exec-root "$R/exec" --pidfile "$R/docker.pid" >"$D/dockerd.log" 2>&1 &
	DPID=$!
	i=0
	until $DOCKER version >/dev/null 2>&1; do
		i=$((i+1)); [ $i -lt 30 ] || { tail -n 15 "$D/dockerd.log"; echo "FAIL rootless Docker daemon of $OU did not start"; exit 1; }
		sleep 1
	done
	pass "rootless Docker daemon runs as $OU"
fi
[ -S "$SOCKET" ] || { echo "FAIL no Docker socket at $SOCKET"; exit 1; }
echo "daemon: $SOCKET $($DOCKER info --format '{{.SecurityOptions}}')"
if [ "$MODE" = rootless ]; then
	has "daemon is rootless" "$($DOCKER info --format '{{.SecurityOptions}}')" "rootless"
	is "daemon socket owned by $OU" "$(stat -c %U "$SOCKET")" "$OU"
fi

asou "$OGID" tee "$D/cfg/openshell/gateway.toml" >/dev/null <<EOF
[openshell]
version = 2

[openshell.gateway]
compute_driver = "docker"
policy_validation_failure_mode = "fail_closed"

[openshell.drivers.docker]
socket_path = "$SOCKET"
allow_driver_config = true
enable_bind_mounts = true

[openshell.drivers.docker.resource_admission]
enabled = false
EOF
# an input folder as zero stages it: under /var/lib/zero/input, read-only bind mount (ADR 0039)
chown "$LU" "$IN"; chmod 755 "$IN"; echo from-host > "$IN/note.txt"; chmod 644 "$IN/note.txt"
MOUNT="{\"docker\":{\"mounts\":[{\"type\":\"bind\",\"source\":\"$IN\",\"target\":\"/tmp/zero-input\",\"read_only\":true}]}}"
asou "$OGID" "$G" config preflight >"$D/preflight.log" 2>&1 || { cat "$D/preflight.log"; echo "FAIL gateway config"; exit 1; }
asou "$OGID" "$G" generate-certs --output-dir "$D/tls" --server-san host.openshell.internal >"$D/certs.log" 2>&1
asou "$GROUPS_" "$G" >"$D/gw.log" 2>&1 &
PID=$!
sleep 3
has "gateway uses the Docker driver" "$(grep 'Using compute driver' "$D/gw.log" | tail -n 1)" "docker"
is "gateway runs as $OU" "$(ps -o user= -p "$PID" | tr -d ' ')" "$OU"
grp=$(grep '^Groups:' "/proc/$PID/status" | cut -f2)
echo "gateway groups: $grp"
if [ "$MODE" = rootful ]; then
	has "rootful: gateway process is in group docker (what zero doctor warns about)" " $grp" " $(getent group docker | cut -d: -f3) "
else
	case " $grp" in *" $(getent group docker | cut -d: -f3) "*) fail "rootless: gateway not in group docker" "$grp";; *) pass "rootless: gateway not in group docker";; esac
fi

# gateway add makes the new gateway the active one and writes its own
# certificates; put the active one back and copy ours afterwards.
aslu openshell gateway add --local --name $NAME "https://127.0.0.1:$PORT" >/dev/null 2>&1
aslu openshell gateway select openshell >/dev/null 2>&1
aslu mkdir -p "$REG/mtls"
install -o "$LU" -m 600 "$D/tls/ca.crt" "$REG/mtls/ca.crt"
install -o "$LU" -m 600 "$D/tls/client/tls.crt" "$REG/mtls/tls.crt"
install -o "$LU" -m 600 "$D/tls/client/tls.key" "$REG/mtls/tls.key"

POLICY=$(cd ../../internal/zero && pwd)/policy.yaml
i=0
until O sandbox create --name $NAME --from "$IMAGE" --policy "$POLICY" --driver-config-json "$MOUNT" --detach >>"$D/create.log" 2>&1; do
	i=$((i+1)); [ $i -lt 5 ] || { cat "$D/create.log"; tail -n 8 "$D/gw.log"; echo "FAIL sandbox create"; exit 1; }
	sleep 3
done
i=0
until O sandbox list 2>/dev/null | grep -q "$NAME.*Ready"; do
	i=$((i+1)); [ $i -lt 120 ] || { O sandbox list; tail -5 "$D/gw.log"; echo "FAIL sandbox not ready after 6 min"; exit 1; }
	sleep 3
done
pass "sandbox ready"

has "agent is not root" "$(O sandbox exec $NAME -- id 2>&1)" "uid=1000"
is "host outside the policy refused" "$(O sandbox exec $NAME -- curl -s -o /dev/null -m 10 -w '%{http_code}' https://example.com 2>/dev/null || true)" 000
has "input folder readable in the sandbox" "$(O sandbox exec $NAME -- cat /tmp/zero-input/note.txt 2>&1)" "from-host"
O sandbox exec $NAME -- touch /tmp/zero-input/new.txt >/dev/null 2>&1 || true
is "input folder not writable from the sandbox" "$(ls "$IN" | tr '\n' ' ')" "note.txt "

n=$($DOCKER ps --filter "name=openshell-default--$NAME" --format '{{.Names}}' | wc -l)
is "two containers at this daemon (workload, supervisor)" "$n" 2
for c in $($DOCKER ps --filter "name=openshell-default--$NAME" --format '{{.Names}}'); do
	s=$($DOCKER inspect "$c" --format 'user={{.Config.User}} priv={{.HostConfig.Privileged}} add={{.HostConfig.CapAdd}} drop={{.HostConfig.CapDrop}} opt={{.HostConfig.SecurityOpt}}')
	case $c in *supervisor) k=supervisor;; *) k=workload;; esac
	echo "$k: $s"
	has "$k: not privileged" "$s" "priv=false"
	has "$k: every capability dropped" "$s" "drop=\[ALL\]"
	has "$k: no new privileges" "$s" "no-new-privileges"
	case $s in *"user= "*|*"user=0"*|*"user=root"*) fail "$k: not root" "$s";; *) pass "$k: not root";; esac
	# who the container's main process is on the host
	hp=$($DOCKER inspect "$c" --format '{{.State.Pid}}')
	if [ "$MODE" = rootless ]; then
		# the pid is inside the daemon's namespace; look the process up by its container id instead
		hu=$(ps -eo uid=,args= | grep "[c]ontainerd-shim.*$($DOCKER inspect "$c" --format '{{.Id}}')" | awk '{print $1}' | head -n 1)
		is "$k: its runtime process on the host is $OU, not root" "$hu" "$OUID"
	else
		hu=$(ps -o uid= -p "$hp" | tr -d ' ')
		case $hu in 0|"") fail "$k: host uid not root" "uid $hu";; *) pass "$k: host uid $hu, not root";; esac
	fi
done

echo "fails: $fails"
[ $fails -eq 0 ]
