#!/bin/sh
# Containment harness (host side). Creates sandbox, runs probe.sh inside, prints table.
# Usage: tests/containment/run.sh [sandbox-name] [image]
set -eu
cd "$(dirname "$0")/../.."
NAME="${1:-f02}"; IMAGE="${2:-docker.io/library/golang:1.25}"
CANARY="z3r0-canary-$(date +%s)"
CFILE="$HOME/.z3r0-canary"
echo "$CANARY" > "$CFILE"
trap 'rm -f "$CFILE"; scripts/sandbox-down.sh "$NAME" >/dev/null 2>&1 || true' EXIT

scripts/sandbox-up.sh "$NAME" tests/containment >/dev/null
rc=0
openshell sandbox exec -n "$NAME" -- sh /sandbox/containment/probe.sh "$HOME" "$CANARY" "$(id -u)" || rc=1

# host side: sandbox container may only bind-mount the openshell-sandbox binary (gateway enable_bind_mounts=false)
id=$(scripts/podman-gw.sh ps --format '{{.Names}}' | grep "^openshell-default--$NAME-")
bad=$(scripts/podman-gw.sh inspect "$id" --format '{{range .Mounts}}{{.Type}}:{{.Destination}}{{"\n"}}{{end}}' \
  | grep '^bind:' | grep -v '^bind:/opt/openshell/bin/openshell-sandbox$' || true)
if [ -z "$bad" ]; then echo "PASS host:no-extra-bind-mounts"; else echo "FAIL host:extra-bind-mounts $bad"; rc=1; fi
exit $rc
