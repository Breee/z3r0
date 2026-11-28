# PoC 34: a sandbox under gVisor

Issue #180. Caveat C64.

## Question

Does an OpenShell sandbox start when the container engine runs containers with gVisor (`runsc`)? Can the OCI runtime be chosen per sandbox?

## Result (2026-11-28, OpenShell 0.1.8, gVisor `runsc` release-20251020.0, Docker 29.5.2 rootless)

No, to both.

| Check | Result |
|---|---|
| Rootless Docker daemon with `default-runtime: runsc` starts, a container runs under gVisor | yes |
| `openshell sandbox create` against that daemon | fails: `ControlSupervisorStartFailed: Docker sandbox exited before supervisor became ready` |
| Workload container | runtime `runsc`, exit 1, log `/proc/self/status is missing CapAmb` |
| Supervisor container | runtime `runsc`, starts, stopped (exit 143) |
| OCI runtime setting in the podman or Docker driver | none |

## Findings

- gVisor's `/proc/self/status` has `CapInh`, `CapPrm`, `CapEff`, `CapBnd`, and no `CapAmb` line. OpenShell's check that the sandbox holds no capabilities reads all five and fails on a missing one (`crates/openshell-sandbox/src/main.rs:120-135`, `boundary_server.rs:3877`).
- The podman and Docker drivers have no setting for the OCI runtime. Only the Kubernetes driver has one (`runtime_class_name`). Under podman and Docker the runtime is the engine's default, for every container of that engine. A second runtime therefore means a second engine and a second gateway.
- Upstream has no issue for this symptom. NVIDIA/OpenShell#981 and #1305 say gVisor needs the sandbox binary split into proxy and runtime first.
- zero does not offer gVisor. No workaround. Upstream draft: `ai-docs/upstream/openshell-gvisor-capamb.md`.

## Sources

- OpenShell v0.1.8: `crates/openshell-sandbox/src/main.rs`, `crates/openshell-driver-docker/src/lib.rs`, `crates/openshell-driver-podman`, `docs/how-it-works/sandboxes/runtimes.mdx`
- gVisor: `runsc --help` (`-rootless`, `-ignore-cgroups`), https://gvisor.dev/docs/user_guide/rootless/
- Docker: `dockerd --help` (`--default-runtime`, `--add-runtime`), `dockerd-rootless.sh`
- NVIDIA/OpenShell#981, NVIDIA/OpenShell#1305

## Limits

- Not tested: podman with `runsc`; `runsc -platform kvm`; what fails after the capability check (network namespace, seccomp, Landlock).
- The gateway ran as the login user, not as `openshell`.

## Run

    pocs/34-gvisor-runtime/run.sh

Starts a throwaway rootless Docker daemon of the calling user (own socket and data under `~/.cache/zero-poc34`), runs PoC 29's flow against it, removes everything. No sudo. Not at the same time as PoC 29. `fails: 0` means the result above was seen again.
