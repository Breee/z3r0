# Recommended host setup

Recommendations for a safer machine. Not required by zero; zero's
guarantees come from the sandbox (`zero install`, `zero doctor`).

- Login user not in the `docker` group (docker group = root).
- OpenShell client key (`~/.config/openshell/gateways/*/mtls/tls.key`) mode 0600 (`zero doctor` checks).
- Disk quota or separate filesystem for the `openshell` user's podman storage, so a sandbox cannot fill the disk.
- Full-disk encryption, screen lock, OS updates.
