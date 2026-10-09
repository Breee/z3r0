#!/bin/sh
# Create a sandbox and upload a local repo to /sandbox/<basename>.
# Usage: scripts/sandbox-up.sh <name> <repo-dir> [image]  (env POLICY=<file> optional)
set -eu
NAME="$1"; REPO="$2"; IMAGE="${3:-localhost/z3r0-dev:latest}"
openshell sandbox create --name "$NAME" --from "$IMAGE" --detach ${POLICY:+--policy "$POLICY"}
openshell sandbox upload "$NAME" "$REPO" /sandbox
