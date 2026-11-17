---
status: proposed
date: 2026-11-17
deciders: bree
requirements: [Z-INV-1, Z-INV-4]
---

# 0039 — Input folders are a filtered host copy, bind-mounted read-only

## Context

Workspace spec §5.3/§6 (#77): a folder that is not a forge repo is input: the agent reads it and cannot change it; secret files are never copied.

The spec said "read-only (filesystem policy)". OpenShell 0.1.8 cannot do that (caveat C52):

- The filesystem policy is fixed at sandbox start (C29), so a folder added later cannot be made read-only.
- `sandbox upload` and `sandbox exec` run as the sandbox user under the same Landlock rules. Every path zero can copy to is a path the agent can write.

OpenShell has a documented option: read-only bind mounts with the podman driver, passed at create with `--driver-config-json`. It needs three gateway settings that `zero install` turns off or leaves off today:

```
[openshell.drivers.podman]
allow_driver_config = true
enable_bind_mounts = true
[openshell.drivers.podman.resource_admission]
enabled = false
```

Probe (2026-11-17, podman gateway, sandbox user uid 1000): the mounted files are readable; write, delete, chmod fail with `Read-only file system`; remount fails with `must be superuser to use mount`; a file added on the host is visible in the running sandbox at once.

## Decision

- zero keeps a filtered copy of every input folder on the host: `/var/lib/zero/input/<sandbox>/<folder>`. Excluded files (§6) and symlinks are not copied. `zero up` refreshes the copy and prints the summary.
- The sandbox gets one bind mount, set at create: `/var/lib/zero/input/<sandbox>` → `/tmp/zero-input`, read-only. Inputs appear as `/tmp/zero-input/<folder>`. Because the directory is mounted, not the single folders, input folders added or removed later need no recreate.
- The original folder is never mounted. Only the filtered copy is.
- `zero install` writes the three gateway settings and creates `/var/lib/zero/input` (owner: the installing user; group: the gateway user's group; mode 0750). Files below are world-readable, because the sandbox user maps to `nobody` on the host; the parent directory keeps other local users out.
- zero mounts nothing below the staging directory (bind mounts are recursive, upstream issue 4579).
- Sandboxes created before this change have no mount: `zero up` says so and asks for `zero rm && zero up` when the workspace has an input folder.
- Podman driver only. The VM driver is out of scope (3 mounts on x86_64; a compromised guest kernel is not confined to the mounted directory).

## Consequences

- "Agent write fails" is enforced by the kernel (read-only mount, agent is not root), not by a policy the agent's own user is subject to.
- The agent gains nothing: it cannot ask for mounts. That needs the gateway client key, which is not in the sandbox.
- The gateway is weaker against a process that holds the user's gateway client key:
  - it can bind any path the gateway user can read into a sandbox, also read-write. This includes the gateway user's home (gateway TLS keys, stored GitLab and Copilot credentials), which the user cannot read today;
  - with admission off it can attach any existing podman volume, including another sandbox's workspace;
  - it can set other podman driver options.
  Such a process already has the user's `glab` login and can create sandboxes. The new part is read access to the gateway user's files.
- Input data exists twice on the host. `zero rm` deletes the copy.
- Refresh is not atomic: during `zero up` the agent can see a half-updated folder.
- Size limit (100 MiB, `--max-input`) is checked on the host before copying.
- Not tested: whether a sandbox process can reach the gateway port; the full list of podman driver options that `allow_driver_config` opens; behaviour with SELinux enforcing.
- Rejected: writable copy inside the sandbox that `zero up` overwrites (agent write would succeed); labelled podman volume (belongs to the gateway user, `zero up` would need sudo); waiting for an upstream read-only upload (`ai-docs/upstream/openshell-read-only-upload.md`).
