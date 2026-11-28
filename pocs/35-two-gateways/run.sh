#!/bin/sh
# PoC 35 (#180): two OpenShell gateways side by side, one per runtime.
# Starts two throwaway gateways as the calling user (podman driver on PORT_A,
# Docker driver on PORT_B), makes one sandbox on each at the same time and
# checks that each sandbox lives in its own engine and is picked by
# `openshell --gateway NAME`. Removes everything again.
# Needs: OpenShell 0.1.8 as installed by zero, the caller's podman socket and
# rootless Docker socket. Must not run together with PoC 29 or 34 (same port).
set -eu
cd "$(dirname "$0")"

RUN=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}
SOCK_A=${SOCK_A:-$RUN/podman/podman.sock}
SOCK_B=${SOCK_B:-$RUN/docker.sock}
PORT_A=${PORT_A:-17690}
PORT_B=${PORT_B:-17691}
IMAGE=${IMAGE:-ghcr.io/breee/z3r0-dev@sha256:a6d406ae281bd0065f873b77a3521316f0d687df4cde3465298dc3163dfcf1af}
# The caller's own podman may have an image allowlist (policy.json) without
# zero's image; IMAGE_A empty means OpenShell's default sandbox image.
IMAGE_A=${IMAGE_A-}
LIB=${LIB:-/usr/local/lib/zero/openshell-0.1.8}
G=$LIB/openshell-gateway
A=poc35-podman
B=poc35-docker
D=$(mktemp -d)
CFG=${XDG_CONFIG_HOME:-$HOME/.config}/openshell/gateways
PIDS=

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
is() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "got $2, want $3"; fi; }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }

cleanup() {
	for n in $A $B; do openshell --gateway $n sandbox delete $n >/dev/null 2>&1 || true; done
	sleep 5
	for p in $PIDS; do kill "$p" 2>/dev/null || true; done
	sleep 1
	for p in $PORT_A $PORT_B; do
		ss -ltn 2>/dev/null | grep -q ":$p " && echo "CLEANUP FAILED: something still listens on port $p"
	done
	for n in $A $B; do openshell gateway remove $n >/dev/null 2>&1 || true; rm -rf "$CFG/$n"; done
	openshell gateway select openshell >/dev/null 2>&1 || echo "CLEANUP FAILED: active gateway not put back to openshell"
	rm -rf "$D"
	left=$(podman ps -aq --filter "name=$A" | wc -l)
	[ "$left" = 0 ] || echo "CLEANUP FAILED: $left containers of $A left in podman"
	left=$(docker -H "unix://$SOCK_B" ps -aq --filter "name=$B" | wc -l)
	[ "$left" = 0 ] || echo "CLEANUP FAILED: $left containers of $B left in Docker"
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

for s in "$SOCK_A" "$SOCK_B"; do [ -S "$s" ] || { echo "FAIL no socket at $s"; exit 1; }; done

# start NAME DRIVER SOCKET PORT: one gateway with its own config, state and certificates
start() {
	d=$D/$1
	mkdir -p "$d/cfg/openshell" "$d/state" "$d/tls"
	cat > "$d/cfg/openshell/gateway.toml" <<EOF
[openshell]
version = 2

[openshell.gateway]
compute_driver = "$2"
policy_validation_failure_mode = "fail_closed"

[openshell.drivers.$2]
socket_path = "$3"
EOF
	(export XDG_CONFIG_HOME="$d/cfg" XDG_STATE_HOME="$d/state" OPENSHELL_LOCAL_TLS_DIR="$d/tls" \
		OPENSHELL_TELEMETRY_ENABLED=false OPENSHELL_SERVER_PORT="$4" \
		OPENSHELL_GATEWAY_CONFIG="$d/cfg/openshell/gateway.toml"
	"$G" config preflight >"$d/preflight.log" 2>&1 || { cat "$d/preflight.log"; exit 1; }
	"$G" generate-certs --output-dir "$d/tls" --server-san host.openshell.internal >"$d/certs.log" 2>&1
	exec "$G" >"$d/gw.log" 2>&1) &
	PIDS="$PIDS $!"
}
# register NAME PORT: the client side; gateway add makes the new gateway the
# active one and writes its own certificates, so put ours in afterwards.
register() {
	openshell gateway add --local --name "$1" "https://127.0.0.1:$2" >/dev/null 2>&1
	openshell gateway select openshell >/dev/null 2>&1
	mkdir -p "$CFG/$1/mtls"
	cp "$D/$1/tls/ca.crt" "$CFG/$1/mtls/ca.crt"
	cp "$D/$1/tls/client/tls.crt" "$CFG/$1/mtls/tls.crt"
	cp "$D/$1/tls/client/tls.key" "$CFG/$1/mtls/tls.key"
	chmod 600 "$CFG/$1/mtls/"*
}
# create NAME IMAGE: a sandbox named like its gateway
create() {
	i=0
	until openshell --gateway "$1" sandbox create --name "$1" ${2:+--from "$2"} \
		--policy ../../internal/zero/policy.yaml --detach >>"$D/$1/create.log" 2>&1; do
		i=$((i+1)); [ $i -lt 5 ] || { tail -n 5 "$D/$1/create.log"; tail -n 5 "$D/$1/gw.log"; echo "FAIL sandbox create on $1"; exit 1; }
		sleep 3
	done
}
ready() {
	i=0
	until openshell --gateway "$1" sandbox list 2>/dev/null | grep -q "$1.*Ready"; do
		i=$((i+1)); [ $i -lt 120 ] || { echo "FAIL $1 not ready after 6 min"; exit 1; }
		sleep 3
	done
}

start $A podman "$SOCK_A" "$PORT_A"
start $B docker "$SOCK_B" "$PORT_B"
sleep 4
has "gateway A uses podman" "$(grep 'Using compute driver' "$D/$A/gw.log" | tail -n 1)" podman
has "gateway B uses Docker" "$(grep 'Using compute driver' "$D/$B/gw.log" | tail -n 1)" docker
register $A "$PORT_A"
register $B "$PORT_B"
is "the installed gateway stays the active one" "$(openshell gateway list 2>/dev/null | sed -n 's/^\* *\([^ ]*\).*/\1/p')" openshell

create $A "$IMAGE_A"
create $B "$IMAGE"
ready $A
ready $B
pass "both sandboxes ready at the same time"

has "gateway A lists its sandbox" "$(openshell --gateway $A sandbox list 2>&1)" "$A"
has "gateway B lists its sandbox" "$(openshell --gateway $B sandbox list 2>&1)" "$B"
is "gateway A does not know B's sandbox" "$(openshell --gateway $A sandbox list 2>&1 | grep -c "$B" || true)" 0
is "gateway B does not know A's sandbox" "$(openshell --gateway $B sandbox list 2>&1 | grep -c "$A" || true)" 0
is "the installed gateway knows neither" "$(openshell sandbox list 2>&1 | grep -c poc35 || true)" 0

na=$(podman ps --format '{{.Names}}' | grep -c "$A" || true)
nb=$(docker -H "unix://$SOCK_B" ps --format '{{.Names}}' | grep -c "$B" || true)
[ "$na" -ge 1 ] && pass "A's sandbox runs in podman ($na containers)" || fail "A's sandbox runs in podman" "$na"
[ "$nb" -ge 1 ] && pass "B's sandbox runs in Docker ($nb containers)" || fail "B's sandbox runs in Docker" "$nb"
is "none of A in Docker" "$(docker -H "unix://$SOCK_B" ps --format '{{.Names}}' | grep -c "$A" || true)" 0
is "none of B in podman" "$(podman ps --format '{{.Names}}' | grep -c "$B" || true)" 0

for n in $A $B; do
	u=$(openshell --gateway $n sandbox exec $n -- id -u 2>&1 | tail -n 1)
	case $u in 0|*[!0-9]*|"") fail "$n: agent is not root" "$u";; *) pass "$n: agent is not root (uid $u)";; esac
	# The default sandbox image has no curl: use what the image has.
	r=$(openshell --gateway $n sandbox exec $n -- sh -c '
		if command -v curl >/dev/null; then t=curl; curl -s -o /dev/null -m 10 https://example.com
		elif command -v python3 >/dev/null; then t=python3; python3 -c "import urllib.request as u; u.urlopen(\"https://example.com\", timeout=10)" 2>/dev/null
		elif command -v wget >/dev/null; then t=wget; wget -q -T 10 -O /dev/null https://example.com
		elif command -v bash >/dev/null; then t=bash; bash -c "exec 3<>/dev/tcp/example.com/443" 2>/dev/null
		else t=none; true; fi
		echo "$t rc=$?"' 2>&1 | tail -n 1)
	case $r in none*|*" rc=0") fail "$n: host outside the policy refused" "$r";; *" rc="*) pass "$n: host outside the policy refused ($r)";; *) fail "$n: host outside the policy refused" "$r";; esac
done
has "gateway info names the driver: A" "$(openshell --gateway $A gateway info 2>&1)" podman
has "gateway info names the driver: B" "$(openshell --gateway $B gateway info 2>&1)" docker

echo "fails: $fails"
[ $fails -eq 0 ]
