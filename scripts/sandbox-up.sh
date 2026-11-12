#!/bin/sh
# Create a sandbox and upload a local repo to /tmp/<basename>.
# Usage: scripts/sandbox-up.sh <name> <repo-dir> [image]  (env POLICY=<file> optional)
set -eu
NAME="$1"; REPO="$2"; IMAGE="${3:-ghcr.io/breee/z3r0-dev@sha256:a279d03ce95e34d75f71a082c1e85f9b59611fcad2fd4871614403396d8064b0}"
openshell sandbox create --name "$NAME" --from "$IMAGE" --detach ${POLICY:+--policy "$POLICY"}
openshell sandbox upload "$NAME" "$REPO" /tmp
