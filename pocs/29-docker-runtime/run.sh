#!/bin/sh
# PoC 29 (#167): an OpenShell sandbox under Docker, rootless or rootful.
# Starts a throwaway gateway as the calling user with the Docker driver,
# makes one sandbox, checks it, removes everything again.
# Needs: OpenShell 0.1.8 as installed by zero, a Docker daemon at SOCKET.
#   SOCKET=$XDG_RUNTIME_DIR/docker.sock ./run.sh   # rootless (default)
#   SOCKET=/var/run/docker.sock ./run.sh           # rootful
set -eu
cd "$(dirname "$0")"

SOCKET=${SOCKET:-${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/docker.sock}
IMAGE=${IMAGE:-ghcr.io/breee/z3r0-dev@sha256:a6d406ae281bd0065f873b77a3521316f0d687df4cde3465298dc3163dfcf1af}
PORT=${PORT:-17690}
LIB=${LIB:-/usr/local/lib/zero/openshell-0.1.8}
G=$LIB/openshell-gateway
NAME=poc29
D=$(mktemp -d)
# not under /tmp: see README, finding on nosuid/nodev sources under rootless Docker
IN=$(mktemp -d -p "${INPUT_PARENT:-${XDG_CACHE_HOME:-$HOME/.cache}}" zero-poc29.XXXXXX)
REG=${XDG_CONFIG_HOME:-$HOME/.config}/openshell/gateways/$NAME
PID=
DOCKER="docker -H unix://$SOCKET"

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
is() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "got $2, want $3"; fi; }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
O() { openshell --gateway $NAME "$@"; }
gw() { env XDG_CONFIG_HOME="$D/cfg" XDG_STATE_HOME="$D/state" OPENSHELL_LOCAL_TLS_DIR="$D/tls" \
	OPENSHELL_TELEMETRY_ENABLED=false OPENSHELL_SERVER_PORT="$PORT" \
	OPENSHELL_GATEWAY_CONFIG="$D/cfg/openshell/gateway.toml" "$G" "$@"; }

cleanup() {
	O sandbox delete $NAME >/dev/null 2>&1 || true
	[ -n "$PID" ] && kill "$PID" 2>/dev/null || true
	ss -ltn 2>/dev/null | grep -q ":$PORT " && echo "CLEANUP FAILED: something still listens on port $PORT"
	openshell gateway remove $NAME >/dev/null 2>&1 || true
	openshell gateway select openshell >/dev/null 2>&1 || echo "CLEANUP FAILED: active gateway not put back to openshell"
	rm -rf "$REG" "$D" "$IN"
	left=$($DOCKER ps -aq --filter "name=openshell-default--$NAME" | wc -l)
	[ "$left" = 0 ] || echo "CLEANUP FAILED: $left containers of $NAME left at $SOCKET"
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

[ -S "$SOCKET" ] || { echo "FAIL no Docker socket at $SOCKET"; exit 1; }
echo "daemon: $SOCKET $($DOCKER info --format '{{.SecurityOptions}}')"

mkdir -p "$D/cfg/openshell" "$D/state" "$D/tls"
cat > "$D/cfg/openshell/gateway.toml" <<EOF
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
# an input folder as zero mounts it: read-only bind mount (ADR 0039)
chmod 755 "$IN"; echo from-host > "$IN/note.txt"; chmod 644 "$IN/note.txt"
MOUNT="{\"docker\":{\"mounts\":[{\"type\":\"bind\",\"source\":\"$IN\",\"target\":\"/tmp/zero-input\",\"read_only\":true}]}}"
gw config preflight >"$D/preflight.log" 2>&1 || { cat "$D/preflight.log"; echo "FAIL gateway config"; exit 1; }
gw generate-certs --output-dir "$D/tls" --server-san host.openshell.internal >"$D/certs.log" 2>&1
# exec in a subshell, so that PID is the gateway itself and cleanup stops it.
(export XDG_CONFIG_HOME="$D/cfg" XDG_STATE_HOME="$D/state" OPENSHELL_LOCAL_TLS_DIR="$D/tls" \
	OPENSHELL_TELEMETRY_ENABLED=false OPENSHELL_SERVER_PORT="$PORT" \
	OPENSHELL_GATEWAY_CONFIG="$D/cfg/openshell/gateway.toml"; exec "$G" >"$D/gw.log" 2>&1) &
PID=$!
sleep 3
has "gateway uses the Docker driver" "$(grep 'Using compute driver' "$D/gw.log" | tail -n 1)" "docker"

# gateway add makes the new gateway the active one and writes its own
# certificates; put the active one back and copy ours afterwards.
openshell gateway add --local --name $NAME "https://127.0.0.1:$PORT" >/dev/null 2>&1
openshell gateway select openshell >/dev/null 2>&1
mkdir -p "$REG/mtls"
cp "$D/tls/ca.crt" "$REG/mtls/ca.crt"
cp "$D/tls/client/tls.crt" "$REG/mtls/tls.crt"
cp "$D/tls/client/tls.key" "$REG/mtls/tls.key"
chmod 600 "$REG/mtls/"*

i=0
until O sandbox create --name $NAME --from "$IMAGE" --policy ../../internal/zero/policy.yaml --driver-config-json "$MOUNT" --detach >>"$D/create.log" 2>&1; do
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
done

echo "fails: $fails"
[ $fails -eq 0 ]
