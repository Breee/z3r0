# Issue for NVIDIA/OpenShell: podman SPIFFE token-exchange demo fails on v0.1.8

No existing issue found (searched "spiffe podman demo", 2026-11-13). Demo unchanged on `main`.

**What:** `examples/spiffe-token-exchange-demo/podman/demo.sh` fails on v0.1.8 (podman 6.2.0 rootless, Ubuntu 26.04).

**Why:** three places where the example was not updated with the product:

1. `provider-profile.yaml` uses `tls: none` → `profile lint`: `unknown tls value 'unknown(-1)'; omit the field`.
2. `demo.sh:417` and `start-gateway.sh:158` set `grpc_endpoint = "http://${GATEWAY_CONTAINER}:8080"`. Since the supervisor/workload split the supervisor container runs with host networking (`crates/openshell-driver-podman/src/container.rs`, `supervisor.netns.nsmode = "host"`), so the container name does not resolve → `Startup configuration fetch failed after 5 attempts: failed to connect to OpenShell server`, sandbox `ContainerExited`.
3. `sandbox create ... --keep --no-tty -- echo "sandbox ready"` leaves the sandbox in phase `Completed`; every following `sandbox exec` fails with `sandbox ... is not ready (phase: Completed)` → `timed out waiting for alpha`. The alpha/beta/token-issuer URLs are container names too, which the host-network supervisor cannot resolve either.

**Repro:**
```
START_GATEWAY=1 GATEWAY_IMAGE=ghcr.io/nvidia/openshell/gateway:0.1.8 \
SUPERVISOR_IMAGE=ghcr.io/nvidia/openshell/supervisor:0.1.8 \
bash examples/spiffe-token-exchange-demo/podman/demo.sh
```

**How (suggested):** drop `tls: none`; use the published loopback port for `grpc_endpoint` and the demo services; create the sandbox with `--detach`.

The SPIRE part itself works on 0.1.8 once 1 and 2 are changed (supervisor gets a per-sandbox JWT-SVID).
