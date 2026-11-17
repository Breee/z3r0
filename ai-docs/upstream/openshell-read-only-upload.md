# openshell: no way to put files into a sandbox that the agent cannot change

Project: NVIDIA/OpenShell — version 0.1.8 (podman driver). Caveat C52. Feature request. Related: 5041 (live filesystem path updates not enforced), 4579 (bind mounts are recursive). No existing issue found (search "read-only upload", "upload read_only", 2026-11-17).

## What
An operator wants to give the agent reference files (specs, data) that it can read and not modify. Without bind mounts there is no way:

- the filesystem policy is applied at sandbox start only, so a path added later cannot become read-only;
- `sandbox upload` and `sandbox exec` run as the sandbox user under the same Landlock rules, so they can only write where the agent can write.

## Repro
```
openshell sandbox create --name t --from <image> --policy policy.yaml --detach   # read_only: [/var/log], read_write: [/tmp]
openshell sandbox upload t ./specs /var/tmp/specs
```
Actual: `tar: /var/tmp: Cannot open: Permission denied`. Upload to `/tmp/specs` works, and the agent can then change the files.
Expected: a way to place files outside the agent's writable paths.

## Wanted
One of:
- `sandbox upload --read-only` (written by the supervisor, not by the sandbox user, into a path the policy marks read-only);
- filesystem `read_only` paths that can be added to a running sandbox and are enforced.

## Why it matters
The only option today is a bind mount. That needs `allow_driver_config`, `enable_bind_mounts` and `resource_admission.enabled = false` for the whole gateway, which lets every client bind any path the gateway user can read.
