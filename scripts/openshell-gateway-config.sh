#!/bin/sh
# Pin OpenShell gateway to rootless Podman, no bind mounts, telemetry off. Run before/after install.
set -eu
mkdir -p "$HOME/.config/openshell" "$HOME/.config/systemd/user/openshell-gateway.service.d"
cat > "$HOME/.config/openshell/gateway.toml" <<TOML
[openshell]
version = 2

[openshell.gateway]
compute_driver = "podman"
policy_validation_failure_mode = "fail_closed"

[openshell.drivers.podman]
socket_path = "$XDG_RUNTIME_DIR/podman/podman.sock"
enable_bind_mounts = false
TOML
printf '[Service]\nEnvironment=OPENSHELL_TELEMETRY_ENABLED=false\n' \
  > "$HOME/.config/systemd/user/openshell-gateway.service.d/z3r0.conf"
systemctl --user daemon-reload
