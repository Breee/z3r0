#!/bin/sh
# P1: run the OpenShell gateway as a rootless podman container (Quadlet) under user openshell,
# next to the native gateway. Gateway name openshell-ctr, port 17672.
# Usage: sudo pocs/07-gateway-container/setup.sh <login-user>   remove: ... <login-user> --remove
set -eu
LU="$1"; OU=openshell; PORT=17672; cd /
IMG=ghcr.io/nvidia/openshell/gateway:0.1.4@sha256:bdb4fe248070032c88be78f029dad9325cb698480e90486e1413a674a5d8edc8
LUID=$(id -u "$LU"); OUID=$(id -u "$OU"); OH=$(getent passwd "$OU" | cut -d: -f6); LH=$(getent passwd "$LU" | cut -d: -f6)
D="$OH/.local/share/openshell-gateway-ctr"; Q="$OH/.config/containers/systemd"
as() { u=$1; id=$2; h=$3; shift 3; sudo -u "$u" env HOME="$h" XDG_RUNTIME_DIR=/run/user/$id XDG_CONFIG_HOME="$h/.config" "$@"; }

if [ "${2:-}" = "--remove" ]; then
  as "$OU" "$OUID" "$OH" systemctl --user stop openshell-gateway-ctr.service || true
  rm -f "$Q/openshell-gateway-ctr.container"; as "$OU" "$OUID" "$OH" systemctl --user daemon-reload
  as "$LU" "$LUID" "$LH" openshell gateway remove openshell-ctr || true
  echo "removed (state kept in $D)"; exit 0
fi

as "$OU" "$OUID" "$OH" systemctl --user enable --now podman.socket
as "$OU" "$OUID" "$OH" mkdir -p "$D" "$Q"
[ -f "$D/tls/ca.crt" ] || as "$OU" "$OUID" "$OH" podman run --rm --userns=keep-id:uid=1000,gid=1000 \
  -v "$D:$D" -e HOME="$D" "$IMG" generate-certs --output-dir "$D/tls"
as "$OU" "$OUID" "$OH" tee "$D/gateway.toml" >/dev/null <<TOML
[openshell]
version = 2

[openshell.gateway]
compute_driver = "podman"
policy_validation_failure_mode = "fail_closed"

[openshell.drivers.podman]
grpc_endpoint = "https://127.0.0.1:$PORT"
TOML
as "$OU" "$OUID" "$OH" tee "$Q/openshell-gateway-ctr.container" >/dev/null <<UNIT
[Unit]
Description=OpenShell gateway (container, P1)

[Container]
Image=$IMG
ContainerName=openshell-gateway-ctr
UserNS=keep-id:uid=1000,gid=1000
PublishPort=127.0.0.1:$PORT:8080
Volume=$D:$D
Volume=/run/user/$OUID/podman/podman.sock:/var/run/podman.sock
Environment=HOME=$D XDG_DATA_HOME=$D
Environment=OPENSHELL_LOCAL_TLS_DIR=$D/tls
Environment=OPENSHELL_GATEWAY_CONFIG=$D/gateway.toml
Environment=OPENSHELL_GATEWAY_NAME=openshell-ctr
Environment=OPENSHELL_COMPUTE_DRIVER=podman
Environment=OPENSHELL_PODMAN_SOCKET=/var/run/podman.sock
Environment=OPENSHELL_DB_URL=sqlite:$D/gateway.db
Environment=OPENSHELL_TELEMETRY_ENABLED=false
NoNewPrivileges=true
DropCapability=ALL

[Service]
Restart=on-failure

[Install]
WantedBy=default.target
UNIT
as "$OU" "$OUID" "$OH" systemctl --user daemon-reload
as "$OU" "$OUID" "$OH" systemctl --user restart openshell-gateway-ctr.service
sleep 8
as "$OU" "$OUID" "$OH" systemctl --user is-active openshell-gateway-ctr.service
echo "--- container mounts:"
as "$OU" "$OUID" "$OH" podman inspect openshell-gateway-ctr --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'

G="$LH/.config/openshell/gateways"; M="$G/openshell-ctr/mtls"
rm -rf "$G/openshell-ctr"
as "$LU" "$LUID" "$LH" openshell gateway add --local --name openshell-ctr "https://127.0.0.1:$PORT"
install -o "$LU" -g "$LU" -m 600 "$D/tls/ca.crt" "$M/ca.crt"
install -o "$LU" -g "$LU" -m 600 "$D/tls/client/tls.crt" "$M/tls.crt"
install -o "$LU" -g "$LU" -m 600 "$D/tls/client/tls.key" "$M/tls.key"
as "$LU" "$LUID" "$LH" openshell gateway select openshell
echo "done: gateway openshell-ctr on :$PORT (active gateway stays openshell)"
