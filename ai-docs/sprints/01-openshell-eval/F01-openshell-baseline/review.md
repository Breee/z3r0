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
