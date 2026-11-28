# Sandbox does not start under gVisor: `/proc/self/status is missing CapAmb`

Project: NVIDIA/OpenShell — version 0.1.8 (gateway and Docker driver; gVisor `runsc` release-20251020.0; Docker 29.5.2 rootless; Ubuntu 26.04). Caveat C64. No existing issue found (web search "OpenShell gVisor CapAmb", 2026-11-28). Related: #981 (gVisor isolation with supervisor and agent in separate pods), #1305 (split `openshell-sandbox` into proxy and runtime crates).

## What

With gVisor as the container runtime of the Docker daemon, `openshell sandbox create` fails. The workload container exits at once with `/proc/self/status is missing CapAmb`. gVisor's `/proc/self/status` has `CapInh`, `CapPrm`, `CapEff` and `CapBnd`, but no `CapAmb` line. The check that the sandbox holds no capabilities reads all five fields and fails when one is missing (`crates/openshell-sandbox/src/main.rs:120-135`, `crates/openshell-sandbox/src/boundary_server.rs:3877`).

## Repro

1. Docker daemon with gVisor as the default runtime (rootless here), `daemon.json`:

   ```json
   {"default-runtime":"runsc","runtimes":{"runsc":{"path":"/usr/local/bin/runsc","runtimeArgs":["--ignore-cgroups"]}}}
   ```

2. `docker run --rm alpine grep '^Cap' /proc/self/status` prints four lines, no `CapAmb`.
3. Gateway with `compute_driver = "docker"` and `[openshell.drivers.docker] socket_path` set to that daemon.
4. `openshell sandbox create --name t --from IMAGE --policy policy.yaml --detach`

Actual: `Error: sandbox entered error phase while provisioning: ControlSupervisorStartFailed: Docker sandbox exited before supervisor became ready`. Log of the workload container: `Error: × /proc/self/status is missing CapAmb`. The supervisor container starts ("Runtime-initiated Sandbox Protocol listener ready") and is stopped.

Expected: one of
- a missing `CapAmb` line counts as an empty ambient set (a kernel without ambient capabilities cannot grant any), and the sandbox starts, or
- the docs name gVisor as not supported by the podman and Docker drivers, and the error says so.

## Why it matters

gVisor is a common second boundary for untrusted code. Today a user who sets it as the engine's runtime gets an error that names neither gVisor nor a way out.

## Also

- The podman and Docker drivers have no setting for the OCI runtime; only the Kubernetes driver has one (`runtime_class_name`). The runtime can only be set as the engine's default, for every container of that engine.
- Not tested: what fails after this check under gVisor (network namespace, seccomp, Landlock); podman with `runsc`; `runsc -platform kvm`.
