#!/bin/sh
# zero install: OpenShell gateway + container runtime under dedicated user `openshell`;
# the login user only gets the mTLS client cert (F09, F15). Idempotent. Run as root.
# $3 is the runtime (ADR 0057): podman, docker-rootless, docker; empty keeps
# the installed one (podman on a first install).
set -eu
LU="$1"; OU=openshell; VER=0.1.8
RT="${3:-}"
[ -n "$RT" ] || RT=$(cat /var/lib/zero/runtime 2>/dev/null || echo podman)
BASE="https://github.com/NVIDIA/OpenShell/releases/download/v$VER"
LIB="/usr/local/lib/zero/openshell-$VER"
cd /

[ "$(uname -m)" = x86_64 ] || { echo "zero install: only x86_64 is pinned" >&2; exit 1; }
case $RT in
  podman) NEED=podman; HINT="Debian/Ubuntu: apt install podman uidmap passt curl";;
  docker-rootless) NEED="dockerd-rootless.sh rootlesskit"; HINT="https://docs.docker.com/engine/security/rootless/";;
  docker) NEED=docker; HINT="https://docs.docker.com/engine/install/";;
  *) echo "zero install: unknown runtime $RT (podman, docker-rootless, docker)" >&2; exit 1;;
esac
for b in $NEED newuidmap newgidmap systemctl curl; do
  command -v "$b" >/dev/null || { echo "zero install: missing $b ($HINT)" >&2; exit 1; }
done
if [ "$RT" = docker ]; then
  [ -S /var/run/docker.sock ] && getent group docker >/dev/null || { echo "zero install: no Docker daemon at /var/run/docker.sock with group docker" >&2; exit 1; }
  cat >&2 <<WARN
zero install: WARNING: runtime docker is the regular (rootful) Docker daemon.
  The user $OU gets its socket, and that equals root on this host (ADR 0057).
  podman or docker-rootless is safer: sudo zero install --podman | --docker-rootless
WARN
fi

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
# only runtime docker puts the gateway's user into group docker; the user
# manager has to start again to pick a change up
INGRP=no; id -nG "$OU" | tr ' ' '\n' | grep -qx docker && INGRP=yes
if [ "$RT" = docker ] && [ $INGRP = no ]; then
  usermod -aG docker "$OU"; systemctl restart "user@$OUID.service"
elif [ "$RT" != docker ] && [ $INGRP = yes ]; then
  gpasswd -d "$OU" docker >/dev/null; systemctl restart "user@$OUID.service"
fi
i=0; while [ ! -S "/run/user/$OUID/bus" ] && [ $i -lt 20 ]; do sleep 0.5; i=$((i+1)); done
as() { sudo -u "$OU" env HOME="$OH" XDG_RUNTIME_DIR="/run/user/$OUID" XDG_CONFIG_HOME="$OH/.config" "$@"; }

# 3. runtime. podman: image allowlist, user socket. docker-rootless: a daemon
# of this user. docker: the system daemon, nothing to start.
as mkdir -p "$OH/.config/containers" "$OH/.config/systemd/user" "$OH/.config/openshell"
[ "$RT" = docker-rootless ] || as systemctl --user disable --now zero-docker.service 2>/dev/null || true
case $RT in
podman)
DRV=podman; SOCK="/run/user/$OUID/podman/podman.sock"; DEP=podman.socket; PTMP=true
as tee "$OH/.config/containers/policy.json" >/dev/null <<'JSON'
{
  "default": [{"type": "reject"}],
  "transports": {
    "docker": {
      "ghcr.io/nvidia": [{"type": "insecureAcceptAnything"}],
      "ghcr.io/anomalyco": [{"type": "insecureAcceptAnything"}],
      "ghcr.io/uniget-org": [{"type": "insecureAcceptAnything"}],
      "ghcr.io/breee/z3r0-dev": [{"type": "insecureAcceptAnything"}],
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
;;
docker-rootless)
# no PrivateTmp: a user unit with it cannot reach the rootless socket (C61)
DRV=docker; SOCK="/run/user/$OUID/docker.sock"; DEP=zero-docker.service; PTMP=false
# as Docker's dockerd-rootless-setuptool.sh writes it
as tee "$OH/.config/systemd/user/zero-docker.service" >/dev/null <<UNIT
[Unit]
Description=Docker, rootless (zero)
Requires=dbus.socket

[Service]
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ExecStart=$(command -v dockerd-rootless.sh)
ExecReload=/bin/kill -s HUP \$MAINPID
TimeoutSec=0
RestartSec=2
Restart=always
StartLimitBurst=3
StartLimitInterval=60s
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
Delegate=yes
Type=notify
NotifyAccess=all
KillMode=mixed

[Install]
WantedBy=default.target
UNIT
as systemctl --user daemon-reload
as systemctl --user enable zero-docker.service
as systemctl --user restart zero-docker.service
i=0; until [ -S "$SOCK" ]; do
  i=$((i+1)); [ $i -lt 60 ] || { echo "zero install: rootless Docker not up (journalctl --user-unit zero-docker as $OU)" >&2; exit 1; }; sleep 1
done
;;
docker)
DRV=docker; SOCK=/var/run/docker.sock; DEP=; PTMP=true
;;
esac

# 4. gateway: driver of the runtime, fail closed, telemetry off. Bind mounts are on
# for the read-only input mount (ADR 0039); they need driver config and
# resource admission off.
as tee "$OH/.config/openshell/gateway.toml" >/dev/null <<TOML
[openshell]
version = 2

[openshell.gateway]
compute_driver = "$DRV"
policy_validation_failure_mode = "fail_closed"

[openshell.drivers.$DRV]
socket_path = "$SOCK"
allow_driver_config = true
enable_bind_mounts = true

[openshell.drivers.$DRV.resource_admission]
enabled = false
TOML
# staging for input directories: written by the login user, read by the gateway user
install -d -m 0755 /var/lib/zero
echo "$RT" > /var/lib/zero/runtime
install -d -o "$LU" -g "$(id -gn "$OU")" -m 0750 /var/lib/zero/input
as tee "$OH/.config/systemd/user/openshell-gateway.service" >/dev/null <<UNIT
[Unit]
Description=OpenShell Gateway (zero)
After=default.target $DEP
${DEP:+Requires=$DEP}

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
PrivateTmp=$PTMP
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
M="$LH/.config/openshell/gateways/openshell/mtls"; S="$OH/.local/state/openshell/tls"
sudo -u "$LU" mkdir -p -m 700 "$M"
install -o "$LU" -g "$LU" -m 600 "$S/ca.crt" "$M/ca.crt"
install -o "$LU" -g "$LU" -m 600 "$S/client/tls.crt" "$M/tls.crt"
install -o "$LU" -g "$LU" -m 600 "$S/client/tls.key" "$M/tls.key"
[ -f "$LH/.config/openshell/gateways/openshell/metadata.json" ] ||
  sudo -u "$LU" openshell gateway add --local --name openshell https://127.0.0.1:17670 >/dev/null
i=0; until sudo -u "$LU" openshell gateway info >/dev/null 2>&1; do
  i=$((i+1)); [ $i -lt 30 ] || { echo "zero install: gateway not reachable" >&2; exit 1; }; sleep 1
done

# 6. zero-cp, the zero service (push gate, token upkeep), as a user unit of the login user
if [ -n "${2:-}" ] && [ "$2" != /usr/local/bin/zero ]; then
  [ -x "$(dirname "$2")/zero-cp" ] || { echo "zero install: zero-cp not found next to $2" >&2; exit 1; }
  install -m 755 "$2" /usr/local/bin/zero
  install -m 755 "$(dirname "$2")/zero-cp" /usr/local/bin/zero-cp
fi
[ -x /usr/local/bin/zero-cp ] || { echo "zero install: /usr/local/bin/zero-cp missing" >&2; exit 1; }
LUID=$(id -u "$LU")
sudo -u "$LU" mkdir -p "$LH/.config/systemd/user"
sudo -u "$LU" tee "$LH/.config/systemd/user/zero-cp.service" >/dev/null <<UNIT
[Unit]
Description=zero-cp, the zero service (push gate, forge token upkeep)

[Service]
ExecStart=/usr/local/bin/zero-cp
Restart=on-failure
RestartSec=5s
# zero-cp parses bytes sent by sandboxes (#100). ProtectKernelModules is not
# possible in a user unit (it needs to drop capabilities).
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=-%h/.local/state/zero -%h/.config/glab-cli
ProtectKernelTunables=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
RestrictNamespaces=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
LockPersonality=yes
MemoryDenyWriteExecute=yes
SystemCallArchitectures=native

[Install]
WantedBy=default.target
UNIT
if [ -S "/run/user/$LUID/bus" ]; then
  lu() { sudo -u "$LU" env XDG_RUNTIME_DIR="/run/user/$LUID" "$@"; }
  # before #119 the service was `zero serve` in zero.service
  lu systemctl --user disable --now zero.service 2>/dev/null || true
  rm -f "$LH/.config/systemd/user/zero.service"
  # a detached one started by an earlier zero up holds the port
  pkill -u "$LU" -f '^[^ ]*zero serve$' 2>/dev/null || true
  pkill -u "$LU" -f '^[^ ]*zero-cp$' 2>/dev/null || true
  lu systemctl --user daemon-reload
  lu systemctl --user enable zero-cp.service
  lu systemctl --user restart zero-cp.service
else
  echo "zero install: no user session bus for $LU; zero up will start zero-cp"
fi
echo "zero install: done (gateway as $OU, runtime $RT, OpenShell $VER). Next: zero doctor"
