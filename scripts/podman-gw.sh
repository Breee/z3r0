#!/bin/sh
# podman of the gateway user (default openshell, F09). Falls back to own podman if that user does not exist.
U="${GW_USER:-openshell}"
id "$U" >/dev/null 2>&1 || exec podman "$@"
cd / && exec sudo -u "$U" env HOME="$(getent passwd "$U" | cut -d: -f6)" XDG_RUNTIME_DIR="/run/user/$(id -u "$U")" podman "$@"
