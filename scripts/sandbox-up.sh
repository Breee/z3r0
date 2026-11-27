#!/bin/sh
# Create a sandbox and upload a local repo to /tmp/<basename>.
# Usage: scripts/sandbox-up.sh <name> <repo-dir> [image]  (env POLICY=<file> optional)
set -eu
NAME="$1"; REPO="$2"; IMAGE="${3:-ghcr.io/breee/z3r0-dev@sha256:a6d406ae281bd0065f873b77a3521316f0d687df4cde3465298dc3163dfcf1af}"
openshell sandbox create --name "$NAME" --from "$IMAGE" --detach ${POLICY:+--policy "$POLICY"}
openshell sandbox upload "$NAME" "$REPO" /tmp
