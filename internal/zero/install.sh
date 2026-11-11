#!/bin/sh
# zero install: OpenShell gateway + rootless podman under dedicated user `openshell`;
# the login user only gets the mTLS client cert (F09, F15). Idempotent. Run as root.
set -eu
LU="$1"; OU=openshell; VER=0.1.8
BASE="https://github.com/NVIDIA/OpenShell/releases/download/v$VER"
LIB="/usr/local/lib/zero/openshell-$VER"
cd /

[ "$(uname -m)" = x86_64 ] || { echo "zero install: only x86_64 is pinned" >&2; exit 1; }

# 1. pinned OpenShell binaries (sha256 pinned in zero, see C41)
if [ ! -x "$LIB/openshell-gateway" ]; then
  T=$(mktemp -d)
  curl -fsSL -o "$T/cli.tgz" "$BASE/openshell-x86_64-unknown-linux-musl.tar.gz"
  curl -fsSL -o "$T/gw.tgz" "$BASE/openshell-gateway-x86_64-unknown-linux-gnu.tar.gz"
  sha256sum -c - <<SUM
87f5c41d396dcd36b4c30a7b5e6f41643a656cb31a82b0edbb00319c9a719c2d  $T/cli.tgz
d2e5e7f80811b7cfbad0d48f0b561ea5e301401d311e4402bbfc419b3d08cbd3  $T/gw.tgz
SUM
  mkdir -p "$LIB"
  tar -xzf "$T/cli.tgz" -C "$LIB" openshell
  tar -xzf "$T/gw.tgz" -C "$LIB" openshell-gateway
  chmod 755 "$LIB/openshell" "$LIB/openshell-gateway"
  rm -rf "$T"
fi
ln -sfn "$LIB/openshell" /usr/local/bin/openshell

# 2. dedicated user with subuids, lingering user manager
id "$OU" >/dev/null 2>&1 || useradd -m -s /usr/sbin/nologin "$OU"
grep -q "^$OU:" /etc/subuid || usermod --add-subuids 200000-265535 "$OU"
grep -q "^$OU:" /etc/subgid || usermod --add-subgids 200000-265535 "$OU"
loginctl enable-linger "$OU"
OUID=$(id -u "$OU"); OH=$(getent passwd "$OU" | cut -d: -f6); LH=$(getent passwd "$LU" | cut -d: -f6)
chmod 700 "$OH"
i=0; while [ ! -d "/run/user/$OUID" ] && [ $i -lt 20 ]; do sleep 0.5; i=$((i+1)); done
as() { sudo -u "$OU" env HOME="$OH" XDG_RUNTIME_DIR="/run/user/$OUID" XDG_CONFIG_HOME="$OH/.config" "$@"; }

# 3. rootless podman: image allowlist, user socket
as mkdir -p "$OH/.config/containers" "$OH/.config/systemd/user" "$OH/.config/openshell"
as tee "$OH/.config/containers/policy.json" >/dev/null <<'JSON'
{
  "default": [{"type": "reject"}],
  "transports": {
    "docker": {
      "ghcr.io/nvidia": [{"type": "insecureAcceptAnything"}],
      "ghcr.io/anomalyco": [{"type": "insecureAcceptAnything"}],
      "ghcr.io/uniget-org": [{"type": "insecureAcceptAnything"}],
      "nvcr.io": [{"type": "insecureAcceptAnything"}],
      "docker.io/library": [{"type": "insecureAcceptAnything"}]
    },
    "containers-storage": {"": [{"type": "insecureAcceptAnything"}]},
    "docker-archive": {"": [{"type": "insecureAcceptAnything"}]},
    "oci-archive": {"": [{"type": "insecureAcceptAnything"}]}
  }
}
JSON
as systemctl --user enable --now podman.socket

# 4. gateway: podman driver, fail closed, no bind mounts, telemetry off
as tee "$OH/.config/openshell/gateway.toml" >/dev/null <<TOML
[openshell]
version = 2

[openshell.gateway]
compute_driver = "podman"
policy_validation_failure_mode = "fail_closed"

[openshell.drivers.podman]
socket_path = "/run/user/$OUID/podman/podman.sock"
enable_bind_mounts = false
TOML
as tee "$OH/.config/systemd/user/openshell-gateway.service" >/dev/null <<UNIT
[Unit]
Description=OpenShell Gateway (zero)
After=default.target podman.socket

[Service]
Type=simple
StateDirectory=openshell/gateway
Environment=OPENSHELL_LOCAL_TLS_DIR=%h/.local/state/openshell/tls
Environment=OPENSHELL_TELEMETRY_ENABLED=false
ExecStartPre=$LIB/openshell-gateway config preflight
ExecStartPre=$LIB/openshell-gateway generate-certs --output-dir \${OPENSHELL_LOCAL_TLS_DIR} --server-san host.openshell.internal
ExecStart=$LIB/openshell-gateway
Restart=on-failure
RestartSec=5s
PrivateTmp=true
UMask=0077

[Install]
WantedBy=default.target
UNIT
as systemctl --user daemon-reload
as systemctl --user enable openshell-gateway.service
as systemctl --user restart openshell-gateway.service
i=0; until [ -r "$OH/.local/state/openshell/tls/client/tls.key" ] && as systemctl --user is-active -q openshell-gateway.service; do
  i=$((i+1)); [ $i -lt 30 ] || { echo "zero install: gateway not active" >&2; exit 1; }; sleep 1
done

# 5. login user: register gateway, client cert only
sudo -u "$LU" sh -c 'openshell gateway list 2>/dev/null | grep -qw openshell || openshell gateway add --local --name openshell https://127.0.0.1:17670 >/dev/null'
M="$LH/.config/openshell/gateways/openshell/mtls"; S="$OH/.local/state/openshell/tls"
install -d -o "$LU" -g "$LU" -m 700 "$M"
install -o "$LU" -g "$LU" -m 600 "$S/ca.crt" "$M/ca.crt"
install -o "$LU" -g "$LU" -m 600 "$S/client/tls.crt" "$M/tls.crt"
install -o "$LU" -g "$LU" -m 600 "$S/client/tls.key" "$M/tls.key"
i=0; until sudo -u "$LU" openshell gateway info >/dev/null 2>&1; do
  i=$((i+1)); [ $i -lt 30 ] || { echo "zero install: gateway not reachable" >&2; exit 1; }; sleep 1
done
echo "zero install: done (gateway as $OU, OpenShell $VER). Next: zero doctor"
