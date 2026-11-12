#!/bin/sh
# Add a second OpenShell gateway `openshell-vm` (MicroVM driver) next to the podman one (F16 step 1, D15).
# Usage: sudo scripts/gateway-vm.sh <login-user>      remove: sudo scripts/gateway-vm.sh <login-user> --remove
# Prereq: sudo zero install done (pinned OpenShell in $LIB); openshell in group kvm.
set -eu
LU="$1"; OU=openshell; PORT=17671; cd /
VER=0.1.8; LIB=/usr/local/lib/zero/openshell-$VER
DRV_SHA=6e85ae05b7330520dbdc6589d30fc4825f76be7ff3ca7ed201243fe3eda37418
LUID=$(id -u "$LU"); OUID=$(id -u "$OU"); OH=$(getent passwd "$OU" | cut -d: -f6); LH=$(getent passwd "$LU" | cut -d: -f6)
as() { u=$1; id=$2; h=$3; shift 3; sudo -u "$u" env HOME="$h" XDG_RUNTIME_DIR=/run/user/$id XDG_CONFIG_HOME="$h/.config" "$@"; }

if [ "${2:-}" = "--remove" ]; then
  as "$OU" "$OUID" "$OH" systemctl --user disable --now openshell-gateway-vm.service || true
  as "$LU" "$LUID" "$LH" openshell gateway remove openshell-vm || true
  echo "removed openshell-vm (state kept in $OH/.local/state/openshell-vm)"; exit 0
fi

# 0. pinned VM driver next to the pinned gateway (sha256 from release checksums)
[ -x "$LIB/openshell-gateway" ] || { echo "run sudo zero install first" >&2; exit 1; }
if [ ! -x "$LIB/openshell-driver-vm" ]; then
  T=$(mktemp -d); F=openshell-driver-vm-x86_64-unknown-linux-gnu.tar.gz
  curl -fsSL -o "$T/$F" "https://github.com/NVIDIA/OpenShell/releases/download/v$VER/$F"
  echo "$DRV_SHA  $T/$F" | sha256sum -c >/dev/null
  tar -xzf "$T/$F" -C "$T"; install -m 755 "$T/openshell-driver-vm" "$LIB/openshell-driver-vm"; rm -rf "$T"
fi

# 1. gateway config + user unit for the openshell user (own name, port, state; shared TLS so the login user's client cert works)
as "$OU" "$OUID" "$OH" tee "$OH/.config/openshell/gateway-vm.toml" >/dev/null <<TOML
[openshell]
version = 2

[openshell.gateway]
compute_driver = "vm"
policy_validation_failure_mode = "fail_closed"

[openshell.drivers.vm]
driver_dir = "$LIB"
vcpus = 2
mem_mib = 2048
overlay_disk_mib = 4096
TOML
as "$OU" "$OUID" "$OH" mkdir -p "$OH/.config/systemd/user"
as "$OU" "$OUID" "$OH" tee "$OH/.config/systemd/user/openshell-gateway-vm.service" >/dev/null <<UNIT
[Unit]
Description=OpenShell Gateway (MicroVM driver)
After=default.target podman.socket

[Service]
Type=simple
Environment=XDG_STATE_HOME=%h/.local/state/openshell-vm
Environment=OPENSHELL_LOCAL_TLS_DIR=%h/.local/state/openshell/tls
Environment=OPENSHELL_GATEWAY_CONFIG=%E/openshell/gateway-vm.toml
Environment=OPENSHELL_GATEWAY_NAME=openshell-vm
Environment=OPENSHELL_SERVER_PORT=$PORT
Environment=OPENSHELL_TELEMETRY_ENABLED=false
ExecStartPre=$LIB/openshell-gateway config preflight
ExecStart=$LIB/openshell-gateway
Restart=on-failure
RestartSec=5s
PrivateTmp=true
UMask=0077

[Install]
WantedBy=default.target
UNIT
as "$OU" "$OUID" "$OH" systemctl --user daemon-reload
as "$OU" "$OUID" "$OH" systemctl --user enable --now openshell-gateway-vm.service
sleep 5
as "$OU" "$OUID" "$OH" systemctl --user is-active openshell-gateway-vm.service
# 2. register for the login user (same client cert as gateway `openshell`)
G="$LH/.config/openshell/gateways"; M="$G/openshell-vm/mtls"
rm -rf "$G/openshell-vm"
as "$LU" "$LUID" "$LH" openshell gateway add --local --name openshell-vm "https://127.0.0.1:$PORT"
for f in ca.crt tls.crt tls.key; do install -o "$LU" -g "$LU" -m 600 "$G/openshell/mtls/$f" "$M/$f"; done
as "$LU" "$LUID" "$LH" openshell gateway list
echo "done: gateway openshell-vm on :$PORT; use: openshell -g openshell-vm ..."
