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
openshell sandbox exec -n "$NAME" -- sh /sandbox/containment/probe.sh "$HOME" "$CANARY"
