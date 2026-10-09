#!/bin/sh
# Move the OpenShell gateway + rootless podman from the login user to dedicated user `openshell` (F09, D06).
# Usage: sudo scripts/gateway-dedicated-user.sh <login-user>      revert: sudo scripts/gateway-dedicated-user.sh <login-user> --revert
# Prereq: useradd openshell, loginctl enable-linger openshell, subuid/subgid entries.
set -eu
LU="$1"; OU=openshell; D="$(cd "$(dirname "$0")" && pwd)"; cd /  # openshell cannot enter caller's cwd
LUID=$(id -u "$LU"); OUID=$(id -u "$OU"); OH=$(getent passwd "$OU" | cut -d: -f6); LH=$(getent passwd "$LU" | cut -d: -f6)
as() { u=$1; id=$2; h=$3; shift 3; sudo -u "$u" env HOME="$h" XDG_RUNTIME_DIR=/run/user/$id XDG_CONFIG_HOME="$h/.config" "$@"; }

if [ "${2:-}" = "--revert" ]; then
  as "$OU" "$OUID" "$OH" systemctl --user disable --now openshell-gateway.service podman.socket || true
  as "$LU" "$LUID" "$LH" systemctl --user enable --now openshell-gateway.service
  M="$LH/.config/openshell/gateways/openshell/mtls"; T="$LH/.local/state/openshell/tls"
  for f in ca.crt client/tls.crt client/tls.key; do install -o "$LU" -g "$LU" -m 600 "$T/$f" "$M/$(basename "$f")"; done
  echo "reverted: gateway runs as $LU again"; exit 0
fi

# 1. stop login-user gateway
as "$LU" "$LUID" "$LH" systemctl --user disable --now openshell-gateway.service || true
chmod 700 "$OH"
# 2. podman for openshell: helper dirs, image policy, user socket
as "$OU" "$OUID" "$OH" mkdir -p "$OH/.config/containers"
printf '[engine]\nhelper_binaries_dir=["/usr/local/bin", "/usr/lib/podman"]\n' | as "$OU" "$OUID" "$OH" tee "$OH/.config/containers/containers.conf" >/dev/null
as "$OU" "$OUID" "$OH" sh -s < "$D/podman-policy.sh"
as "$OU" "$OUID" "$OH" sh -s < "$D/podman-user-socket.sh"
# 3. gateway config (podman socket of openshell, no bind mounts, telemetry off) + start
as "$OU" "$OUID" "$OH" sh -s < "$D/openshell-gateway-config.sh"
as "$OU" "$OUID" "$OH" systemctl --user enable --now openshell-gateway.service
sleep 5
as "$OU" "$OUID" "$OH" systemctl --user is-active openshell-gateway.service
# 4. give login user only the client cert (mTLS); nothing else
M="$LH/.config/openshell/gateways/openshell/mtls"; T="$OH/.local/state/openshell/tls"
install -o "$LU" -g "$LU" -m 600 "$T/ca.crt" "$M/ca.crt"
install -o "$LU" -g "$LU" -m 600 "$T/client/tls.crt" "$M/tls.crt"
install -o "$LU" -g "$LU" -m 600 "$T/client/tls.key" "$M/tls.key"
echo "done: gateway runs as $OU; $LU has client cert only"
