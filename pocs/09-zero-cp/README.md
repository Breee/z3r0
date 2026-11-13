# PoC 09 — minimal zero-cp token endpoint (#90, Sprint 05 P3)

Question: can a control plane hand a credential only to a known sandbox identity, without the agent ever holding it?

Result (2026-11-13, OpenShell 0.1.8, SPIRE 1.12.4, podman 6.2.0): **yes**. `run.sh` 12/12 PASS.

| Check | Result |
|---|---|
| sandbox registered in zero-cp calls the test service | 200, request carries the granted token |
| sandbox with a valid SVID but not registered | 502 from the supervisor, nothing forwarded |
| agent calls `/token` itself | 403 by sandbox policy |
| agent looks for the token (env, files, response headers/body) | not found |
| forged / missing assertion | 401 |
| registration removed | next request 502 (`cache_ttl: 0s`) |
| decision log | one JSON line per allow and deny, with SPIFFE ID and reason |

## How it works

- `cmd/zero-cp` (Go, stdlib): `POST /token` verifies the JWT-SVID (signature against the SPIRE JWT bundle, audience, expiry), looks the SPIFFE ID up in an allow file, returns a random test token, logs the decision.
- `profile.yaml`: OpenShell provider with `token_grant` (`client_credentials`, `jwt-spiffe` assertion). The supervisor fetches its SVID, asks zero-cp, injects the token into the outgoing request. Provider created with `--runtime-credentials`: nothing stored in the gateway, no placeholder in the sandbox.
- The supervisor runs on the host network, so the token endpoint is `127.0.0.1`; the agent reaches the test service as `host.openshell.internal`.

## Reproduce

Start SPIRE + throwaway gateway as in `pocs/08-spire-podman/README.md`, then:

```
OPENSHELL_SRC=<OpenShell v0.1.8 clone> pocs/09-zero-cp/run.sh
```

## Limits / not covered

- zero-cp ran as a host process, not as a container.
- The allow file stands in for sessions/epochs and policy (arch §22, §27).
- Test token only: no OpenBao (P4), no GitLab minting or revocation at the forge (P5).
- The JWT bundle is read once at start; SPIRE key rotation is not handled.
- Token endpoint is plain HTTP on loopback; any host process can call it, but needs a valid SVID.
- Same trust boundary as PoC 08: the podman socket owner can obtain any sandbox's SVID (C49).
