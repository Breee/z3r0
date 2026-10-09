# F01 — Review

## Step 1: Read docs + check host prereqs
```sh
gh release list -R NVIDIA/OpenShell -L 3
git clone --depth 1 --branch v0.1.2 https://github.com/NVIDIA/OpenShell /tmp/OpenShell
less /tmp/OpenShell/docs/about/support-matrix.mdx /tmp/OpenShell/docs/about/run-your-first-agent.mdx
uname -r                                # expect >= 6.2
cat /sys/kernel/security/lsm            # expect contains landlock
stat -fc %T /sys/fs/cgroup              # expect cgroup2fs
podman --version                        # expect 5.x
systemctl --user is-active podman.socket  # expect active
```
Expected: all pass. Actual (2026-10-09): kernel 7.0.0, landlock yes, cgroup2fs, **podman missing**.

## Step 2a: Rootless Podman 6.1.3 (manual install) working
Prereqs (user, sudo): AppArmor profile `/etc/apparmor.d/podman-local` for `/usr/local/bin/podman` (`userns,`), `sudo apt install passt aardvark-dns crun`.
```sh
printf '[engine]\nhelper_binaries_dir=["/usr/local/bin", "/usr/lib/podman"]\n' > ~/.config/containers/containers.conf
./scripts/podman-user-socket.sh      # installs upstream user units, enables socket
./scripts/podman-policy.sh           # reject-by-default image policy
podman --remote info --format 'rootless={{.Host.Security.Rootless}} cgroup={{.Host.CgroupsVersion}} ver={{.Version.Version}}'
podman run --rm docker.io/library/alpine:3 true; echo $?
podman pull quay.io/podman/hello
```
Expected: `active`; `rootless=true cgroup=v2 ver=6.1.3`; `0`; last pull `rejected by policy`.

## Step 2b: Pin gateway config (before install)
```sh
./scripts/openshell-gateway-config.sh
cat ~/.config/openshell/gateway.toml
cat ~/.config/systemd/user/openshell-gateway.service.d/z3r0.conf
```
Expected: `compute_driver = "podman"`, `enable_bind_mounts = false`, `fail_closed`; drop-in sets `OPENSHELL_TELEMETRY_ENABLED=false`.
