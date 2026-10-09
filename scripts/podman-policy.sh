#!/bin/sh
# Rootless podman image policy: reject by default, allow listed registries (no signature check).
set -eu
mkdir -p "$HOME/.config/containers"
cat > "$HOME/.config/containers/policy.json" <<'JSON'
{
  "default": [{"type": "reject"}],
  "transports": {
    "docker": {
      "ghcr.io/nvidia": [{"type": "insecureAcceptAnything"}],
      "ghcr.io/anomalyco": [{"type": "insecureAcceptAnything"}],
      "nvcr.io": [{"type": "insecureAcceptAnything"}],
      "docker.io/library": [{"type": "insecureAcceptAnything"}]
    },
    "containers-storage": {"": [{"type": "insecureAcceptAnything"}]},
    "docker-archive": {"": [{"type": "insecureAcceptAnything"}]},
    "oci-archive": {"": [{"type": "insecureAcceptAnything"}]}
  }
}
JSON
