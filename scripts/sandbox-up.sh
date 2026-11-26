#!/bin/sh
# Create a sandbox and upload a local repo to /tmp/<basename>.
# Usage: scripts/sandbox-up.sh <name> <repo-dir> [image]  (env POLICY=<file> optional)
set -eu
NAME="$1"; REPO="$2"; IMAGE="${3:-ghcr.io/breee/z3r0-dev@sha256:cf8cde9b1b95f1eee8b5af1cbc2739be8336d323f6aef943cf901a1f64e507d3}"
openshell sandbox create --name "$NAME" --from "$IMAGE" --detach ${POLICY:+--policy "$POLICY"}
openshell sandbox upload "$NAME" "$REPO" /tmp
