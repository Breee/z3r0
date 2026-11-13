# PoC 08 — SPIRE attestation of the supervisor on podman (#83)

Question: does each sandbox supervisor get its own SPIFFE identity on rootless podman, and can the agent or another sandbox get it?

Result (2026-11-13, OpenShell 0.1.8, SPIRE 1.12.4, podman 6.2.0): **yes / no**.

| Check | Result |
|---|---|
| supervisor of s1 fetches JWT-SVID | `sub = spiffe://openshell.local/openshell/sandbox/<s1-id>` |
| supervisor of s2, no entry registered | `PermissionDenied: no identity issued` |
| supervisor of s2 after registration | `sub = …/sandbox/<s2-id>` (never s1) |
| agent in s1 (`sandbox exec`) | `/spiffe-workload-api` does not exist; workload container has no socket mount, network `none` |
| host process outside a container | `no identity issued` |
| any container started with labels `openshell.managed=true` + `openshell.ai/sandbox-id=<s1-id>` and the socket mounted | gets the s1 SVID |

## What this means

- Identity is bound to the sandbox by the SPIRE `docker` workload attestor against the podman socket (selectors: the two labels above). One SPIRE entry per sandbox, created after `sandbox create` (the id is only known then).
- Only the supervisor container gets the socket (read-only bind at `/spiffe-workload-api`). The workload container carries the same labels but cannot reach the socket.
- Trust boundary = whoever controls the podman socket and the socket directory. They can forge the labels. In zero that is the dedicated `openshell` user, not the developer and not the agent.
- The SPIRE agent needs `--pid=host` and the podman socket.

## Reproduce

Uses the upstream demo to start SPIRE + a throwaway gateway (`http://127.0.0.1:18082`); two example bugs must be changed first, see `ai-docs/upstream/openshell-spiffe-podman-demo.md` (C49). The demo then fails at its token-exchange step; SPIRE and gateway stay up (`KEEP_DEMO=1`).

```
D=<OpenShell v0.1.8 clone>/examples/spiffe-token-exchange-demo/podman
START_GATEWAY=1 KEEP_DEMO=1 KEEP_SANDBOX=1 GATEWAY_NAME=spiffe-poc \
  PODMAN_SOCKET=$XDG_RUNTIME_DIR/podman/podman.sock NODE_IMAGE=docker.io/library/node:22-alpine \
  GATEWAY_IMAGE=ghcr.io/nvidia/openshell/gateway:0.1.8 SUPERVISOR_IMAGE=ghcr.io/nvidia/openshell/supervisor:0.1.8 \
  bash $D/demo.sh
O="openshell --gateway spiffe-poc --gateway-endpoint http://127.0.0.1:18082"
$O sandbox create --name s1 --provider spiffe-token-exchange-demo-podman --detach   # same for s2
bash $D/spire/register-sandbox.sh <s1-id>
podman cp openshell-spiffe-demo-spire-agent:/opt/spire/bin/spire-agent .
podman cp spire-agent openshell-supervisor-<id>:/spire-agent
podman exec openshell-supervisor-<id> /spire-agent api fetch jwt \
  -socketPath /spiffe-workload-api/agent.sock -audience poc
$O sandbox exec --name s1 -- ls /spiffe-workload-api
```

## Not covered

Token exchange with the SVID (`token_grant`), SVID use by the real supervisor against zero-cp (P3), SPIRE under the `openshell` user in `zero install`.
