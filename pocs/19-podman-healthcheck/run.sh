#!/bin/sh
# PoC 19 (#169): does the container engine run health checks by itself, so that
# a compose service waiting for `service_healthy` starts?
# ENGINE=podman (default) or docker. WAIT = seconds to watch (default 30).
set -u
cd "$(dirname "$0")"
ENGINE=${ENGINE:-podman}
WAIT=${WAIT:-30}
P=poc19
C="$ENGINE compose -p $P -f compose.yaml"
A=$P-a-1
B=$P-b-1

cleanup() {
	$ENGINE rm -f $A $B >/dev/null 2>&1
	$ENGINE network rm ${P}_default >/dev/null 2>&1
}
trap cleanup EXIT
cleanup

echo "== engine"
bin=$(command -v "$ENGINE")
echo "binary: $bin, $($ENGINE --version)"
if [ "$ENGINE" = podman ]; then
	tags=$(go version -m "$bin" 2>/dev/null | grep -o -- '-tags=[^ ]*')
	echo "build tags: ${tags:-unknown (go not installed)}"
	case ",${tags#-tags=}," in *,systemd,*) echo "systemd tag: yes" ;; *) echo "systemd tag: no" ;; esac
	echo "cgroup manager: $(podman info --format '{{.Host.CgroupManager}}'), events: $(podman info --format '{{.Host.EventLogger}}')"
fi

status() { $ENGINE inspect --format '{{.State.Health.Status}}' $A 2>/dev/null; }
running() { $ENGINE inspect --format '{{.State.Running}}' "$1" 2>/dev/null; }

echo "== compose up (b waits for a healthy), watched for $WAIT s"
$C up -d >up.log 2>&1 &
up=$!
i=0
while [ $i -lt "$WAIT" ]; do
	sleep 5
	i=$((i + 5))
	echo "t=${i}s a health: $(status) b running: $(running $B)"
	[ "$(running $B)" = true ] && break
done
if [ "$ENGINE" = podman ]; then
	echo "health check timers of the user's systemd: $(systemctl --user list-timers --all 2>/dev/null | grep -c "$($ENGINE inspect --format '{{.Id}}' $A)")"
fi
own=no
[ "$(running $B)" = true ] && own=yes
echo "b started by itself: $own"

if [ $own = no ]; then
	echo "== contrast: the check run by hand"
	$ENGINE healthcheck run $A >/dev/null 2>&1
	echo "podman healthcheck run: rc $?, a health: $(status)"
	j=0
	while [ $j -lt 20 ] && [ "$(running $B)" != true ]; do
		sleep 2
		j=$((j + 2))
	done
	echo "b running after the check by hand: $(running $B)"
fi
kill $up 2>/dev/null
wait $up 2>/dev/null
rm -f up.log
echo "result: engine runs health checks by itself: $own"
[ $own = yes ]
