#!/bin/sh
# Create a sandbox and upload a local repo to /tmp/<basename>.
# Usage: scripts/sandbox-up.sh <name> <repo-dir> [image]  (env POLICY=<file> optional)
set -eu
NAME="$1"; REPO="$2"; IMAGE="${3:-ghcr.io/breee/z3r0-dev@sha256:f2e120fbb984ff6efbc157226a82a050e269c333930d50664c58f92820dbb504}"
openshell sandbox create --name "$NAME" --from "$IMAGE" --detach ${POLICY:+--policy "$POLICY"}
openshell sandbox upload "$NAME" "$REPO" /tmp
