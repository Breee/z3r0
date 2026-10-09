#!/bin/sh
# Resource abuse, bounded. Runs on HOST, evidence from the container cgroup. Usage: resources.sh <sandbox>
N="$1"; rc=0
X() { timeout 120 openshell sandbox exec -n "$N" -- sh -c "$1" >/dev/null 2>&1; }
ok() { if [ "$2" = 0 ]; then echo "PASS $1"; else echo "FAIL $1 $3"; rc=1; fi; }
id=$(podman ps --format '{{.Names}}' | grep "^openshell-default--$N-")
CG=/sys/fs/cgroup$(podman inspect "$id" --format '{{.State.CgroupPath}}')
L=$(podman inspect "$id" --format '{{.HostConfig.Memory}} {{.HostConfig.PidsLimit}} {{.HostConfig.NanoCpus}}')
set -- $L; [ "$1" -gt 0 ] && [ "$2" -gt 0 ] && [ "$3" -gt 0 ]; ok res:limits-set $? "($L)"
X 'bash -c "for i in \$(seq 4000); do sleep 10 & done"'; sleep 12      # fork bomb (session dies, sandbox lives)
[ "$(cat $CG/pids.peak)" -le "$(cat $CG/pids.max)" ] && grep -q '^max [1-9]' $CG/pids.events; ok res:fork-capped $? "($(cat $CG/pids.events))"
X 'yes | head -c 1073741824 > /tmp/fill && rm /tmp/fill'                # 1G write, no quota
if [ $? = 0 ]; then echo "GAP  res:no-disk-quota (C22)"; else echo "PASS res:no-disk-quota"; fi
X 'yes | tr -d "\\n" | tail'; sleep 5                       # memory hog (last: may kill sandbox)
for i in $(seq 12); do st=$(podman inspect "$id" --format '{{.State.Status}} {{.State.ExitCode}}' 2>/dev/null); case "$st" in exited*) break;; esac; sleep 5; done
uptime >/dev/null; ok res:host-survives-oom $?
case "$st" in exited*137) echo "GAP  res:oom-kills-sandbox (C23)";; *) X true; ok res:sandbox-survives-oom $? "($st)";; esac
exit $rc
