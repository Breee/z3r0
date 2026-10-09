#!/bin/sh
# Build images/dev into the gateway user's podman storage (needs sudo). Prints image ID.
# Usage: scripts/build-dev-image.sh [tag]   (default localhost/z3r0-dev:latest)
set -eu
TAG="${1:-localhost/z3r0-dev:latest}"
DIR="$(cd "$(dirname "$0")/.." && pwd)"
"$DIR/scripts/podman-gw.sh" build -t "$TAG" - < "$DIR/images/dev/Containerfile"
"$DIR/scripts/podman-gw.sh" image inspect "$TAG" --format '{{.Id}} {{.Size}}'
