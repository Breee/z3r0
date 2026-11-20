# PoC 14 — Kubernetes MCP server behind one shared agentgateway, credential per session (#128)

Question: can one gateway serve several sandboxes and call the cluster with each session's own ServiceAccount, with no cluster credential in the sandbox?

Result (2026-11-20, OpenShell 0.1.8, SPIRE 1.12.4, agentgateway 1.7.0, kubernetes-mcp-server 0.1.3, kind with Kubernetes 1.35, podman): **yes**. `run.sh` 25/25 PASS.

## Chain

```
agent (curl) -> supervisor injects JWT-SVID -> agentgateway :18091
   jwtAuth: verifies the SVID with SPIRE's keys
   extAuthz: asks zero-cp (subject = SPIFFE ID, gateway key) -> session's ServiceAccount token
   mcpAuthorization: tool allow list
   backendAuth: sets the token for this request
-> kubernetes-mcp-server 127.0.0.1:18092 (read_only, kubeconfig without credentials, passes the token on)
-> kind API server (RBAC: Roles zero-k8s-* bound to the session ServiceAccount)
```

Sessions: sandbox a = `zero-p14a` in team-a (`k8s.resource.read`, `k8s.pod.logs`), sandbox b = `zero-p14b` in team-b (`k8s.resource.read`), created with `tests/k8s/session.sh`.

## Result

| Check | Result |
|---|---|
| a lists tools | read tools of the rule; `namespaces_list` hidden, no `pods_delete` |
| a lists pods in team-a, reads a pod log | works |
| a reads Secrets, lists pods in team-b | `forbidden`, cluster names `team-a:zero-p14a` |
| a calls a write tool, a hidden tool | error |
| b lists pods in team-b through the same gateway | works |
| b lists pods in team-a | `forbidden`, cluster names `team-b:zero-p14b` |
| b reads a pod log (capability not granted) | `forbidden` |
| call without identity, call with a cluster token as identity | 401 |
| subject header sent to zero-cp without the gateway key | 403 |
| token in the sandbox (env, files, responses, tool results) | none |
| cluster audit log | a's ServiceAccount: list pods 200 in team-a, 403 in team-b; b's: 200 in team-b |
| a's session stopped (ServiceAccount deleted) | next call fails at the cluster |
| a's session removed in zero-cp | gateway fails closed (`no session`), b still works |

## Findings

- The backend credential is chosen per request (`backendAuth.key.expression: extauthz.backend_token`), not per MCP session: a changed or removed session takes effect at the next call.
- After `jwtAuth` the gateway does not pass the Authorization header to the external authorization service. So the gateway verifies the SVID and sends the subject in a header; zero-cp accepts that header only with the gateway's own key.
- zero-cp is asked on every request (HTTP external authorization is not cached).
- kubernetes-mcp-server rejects the gateway's requests with 403 unless `disable_localhost_protection = true`, because the gateway keeps the sandbox's `Host` header. Its documentation names this setting for a server behind a trusted proxy together with `bind_address = "127.0.0.1"` (default is all interfaces). Both are set.
- The SPIRE agent of the upstream demo stopped after about 2 hours (`rotate agent SVID failed ... PermissionDenied`, join-token attestation). The environment had to be started again.

## Limits

- The tool allow list is the same for both sandboxes; the difference between them comes from the cluster credential only. Rules per session (from capabilities) are not tested.
- agentgateway listens on all interfaces in this PoC.
- The gateway key is a shared secret in a file.
- Client is curl, not opencode. No streams (`logs -f`).
- zero-cp is the PoC from `pocs/09-zero-cp`, sessions are token files written by the script.

## Run

Needs the PoC 08 environment (SPIRE + gateway `spiffe-poc`) and the kind cluster of `tests/k8s`.

```
IMAGE=<image with curl> OPENSHELL_SRC=<OpenShell checkout with the C49 changes> \
AGENTGATEWAY=<binary> K8SMCP=<kubernetes-mcp-server binary> \
ADMIN_KUBECONFIG=… HUMAN_KUBECONFIG=… AGENT_KUBECONFIG=… pocs/14-k8s-mcp/run.sh
```
