#!/bin/sh
# What zero-cp would do with the human's own kubeconfig. Usage:
#   session.sh start NS NAME CAP...   prints a token (capability k8s.pod.logs -> Role zero-k8s-pod-logs)
#   session.sh stop  NS NAME          deletes the service account: token dead, bindings garbage collected
set -eu
op=$1 ns=$2 sa=zero-$3; shift 3
case $op in
start)
  kubectl -n "$ns" create serviceaccount "$sa" >/dev/null
  uid=$(kubectl -n "$ns" get serviceaccount "$sa" -o jsonpath='{.metadata.uid}')
  for cap in "$@"; do
    role=zero-$(echo "$cap" | tr . -)
    kubectl -n "$ns" apply -f - >/dev/null <<Y
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: $sa-$role
  ownerReferences: [{apiVersion: v1, kind: ServiceAccount, name: $sa, uid: $uid}]
roleRef: {apiGroup: rbac.authorization.k8s.io, kind: Role, name: $role}
subjects: [{kind: ServiceAccount, name: $sa, namespace: $ns}]
Y
  done
  kubectl -n "$ns" create token "$sa" --duration 1h ;;
stop)
  kubectl -n "$ns" delete serviceaccount "$sa" ;;
esac
