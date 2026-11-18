#!/bin/sh
# Runs the agent's calls with a session token. Usage: probe.sh KUBECONFIG_WITHOUT_CREDENTIALS TOKEN NS OTHER_NS
k="kubectl --kubeconfig $1 --token $2 --request-timeout 10s"; ns=$3; other=$4
t() { printf '%-38s ' "$*"; out=$($k "$@" 2>&1); if [ $? -eq 0 ]; then echo "ALLOWED  $(echo "$out" | tail -1 | cut -c1-60)"; else echo "denied   $(echo "$out" | grep -o 'cannot [a-z]* resource "[^"]*"\|denied request: .*\|Unauthorized' | head -1 | cut -c1-90)"; fi; }
$k auth whoami -o jsonpath='{.status.userInfo.username}{"\n"}'
t -n "$ns" get pods
t -n "$ns" logs deploy/api --tail 1
t -n "$ns" get deploy api -o name
t -n "$ns" get events -o name
t -n "$ns" get configmap api-config -o name
t -n "$ns" get secret api-db -o name
t -n "$ns" exec deploy/api -- id
t -n "$ns" port-forward deploy/api 18080:80
t -n "$ns" delete pod -l app=api
t -n "$ns" scale deploy api --replicas 2
t -n "$ns" rollout restart deploy api
t -n "$ns" set image deploy api api=busybox:1.37
t -n "$ns" get --raw "/api/v1/namespaces/$ns/pods/api/proxy/"
t -n "$ns" create token deployer
t -n "$ns" create rolebinding x --clusterrole admin --serviceaccount "$ns:default"
t -n "$other" get pods
t get nodes
t get namespaces
