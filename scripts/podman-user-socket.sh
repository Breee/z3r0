#!/bin/sh
# Install upstream Podman user systemd units for a non-packaged podman binary.
set -eu
VER="${PODMAN_VERSION:-v$(podman version --format '{{.Client.Version}}')}"
BIN="$(command -v podman)"
DIR="$HOME/.config/systemd/user"
URL="https://raw.githubusercontent.com/containers/podman/$VER/contrib/systemd/system"
mkdir -p "$DIR"
curl -fsSL "$URL/podman.socket" -o "$DIR/podman.socket"
curl -fsSL "$URL/podman.service.in" | sed "s|@@PODMAN@@|$BIN|" > "$DIR/podman.service"
systemctl --user daemon-reload
systemctl --user enable --now podman.socket
systemctl --user is-active podman.socket
