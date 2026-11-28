#!/bin/sh
# PoC 34 (#180): does an OpenShell sandbox start under gVisor (runsc)?
# Starts a throwaway rootless Docker daemon of the calling user with runsc
# as its default runtime, runs PoC 29's sandbox flow against it, removes
# everything again. No sudo. Must not run at the same time as PoC 29.
# Needs: runsc, dockerd-rootless.sh, rootlesskit, OpenShell 0.1.8 from zero.
set -eu
cd "$(dirname "$0")"

RUNSC=${RUNSC:-$(command -v runsc)}
B=${XDG_CACHE_HOME:-$HOME/.cache}/zero-poc34
R=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/zero-poc34
SOCK=$R/docker.sock
D="docker -H unix://$SOCK"
WPID=

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-300)"; fails=$((fails+1)); }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }

cleanup() {
	[ -n "${LPID:-}" ] && kill "$LPID" 2>/dev/null || true
	[ -r "$R/docker.pid" ] && kill "$(cat "$R/docker.pid")" 2>/dev/null || true
	[ -n "$WPID" ] && kill "$WPID" 2>/dev/null || true
	sleep 2
	rootlesskit rm -rf "$B" "$R" 2>/dev/null || rm -rf "$B" "$R" 2>/dev/null || true
	[ -e "$B" ] || [ -e "$R" ] && echo "CLEANUP FAILED: $B or $R left" || true
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

mkdir -p "$B" "$R"
cat > "$B/daemon.json" <<JSON
{"default-runtime":"runsc","runtimes":{"runsc":{"path":"$RUNSC","runtimeArgs":["--ignore-cgroups"]}}}
JSON
(export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
	DOCKERD_ROOTLESS_ROOTLESSKIT_STATE_DIR="$R/rk"
exec dockerd-rootless.sh --config-file "$B/daemon.json" --host "unix://$SOCK" \
	--data-root "$B/data" --exec-root "$R/exec" --pidfile "$R/docker.pid" >"$B/dockerd.log" 2>&1) &
WPID=$!
i=0
until $D info >/dev/null 2>&1; do
	i=$((i+1)); [ $i -lt 60 ] || { tail -5 "$B/dockerd.log"; echo "FAIL daemon did not start"; exit 1; }
	sleep 1
done
has "daemon's default runtime is runsc" "$($D info --format '{{.DefaultRuntime}}')" runsc
$D pull -q alpine >/dev/null
has "a container runs under gVisor" "$($D run --rm alpine dmesg 2>&1 | head -3)" "Starting gVisor"
caps=$($D run --rm alpine grep '^Cap' /proc/self/status)
echo "$caps" | sed 's/^/  /'
if printf %s "$caps" | grep -q CapAmb; then echo "NOTE gVisor prints CapAmb here"; else pass "gVisor's /proc/self/status has no CapAmb line"; fi

# the driver removes failed containers: keep their logs while PoC 29 runs
(while :; do
	for c in $($D ps -aq 2>/dev/null); do
		n=$($D inspect --format '{{.Name}} rt={{.HostConfig.Runtime}} exit={{.State.ExitCode}}' "$c" 2>/dev/null) || continue
		{ echo "== $n"; $D logs "$c" 2>&1 | tail -5; } > "$B/log.$c" 2>/dev/null || true
	done
	sleep 1
done) &
LPID=$!
out=$(SOCKET=$SOCK ../29-docker-runtime/run.sh 2>&1) && rc=0 || rc=$?
kill "$LPID" 2>/dev/null || true; LPID=
logs=$(cat "$B"/log.* 2>/dev/null | sed 's/\x1b\[[0-9;]*m//g' || true)

if [ "$rc" = 0 ]; then
	echo "RESULT: a sandbox starts under gVisor"
	printf '%s\n' "$out" | grep -E '^(PASS|FAIL|fails)'
else
	echo "RESULT: no sandbox under gVisor"
	has "create fails before the supervisor is ready" "$out" "ControlSupervisorStartFailed"
	has "the workload ran under runsc" "$logs" "rt=runsc"
	has "cause: CapAmb missing" "$logs" "missing CapAmb"
	printf '%s\n' "$logs" | grep -E '^==|CapAmb' | cut -c1-200 | sort -u | sed 's/^/  /'
fi
echo "fails: $fails"
[ $fails -eq 0 ]
