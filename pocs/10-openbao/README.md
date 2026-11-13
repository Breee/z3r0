# PoC 10 — OpenBao as OpenShell credential store (#88)

Question: can the gateway keep provider credentials in OpenBao instead of its own database, without changing how sandboxes use them?

Result (2026-11-13, OpenShell 0.1.8, OpenBao 2.7.2, podman 6.2.0): **yes**. `run.sh` 6/6 PASS.

| Check | Result |
|---|---|
| sandbox calls a service that accepts only the real secret | 200 (placeholder substituted) |
| agent reads `$POC_TOKEN` | placeholder only |
| secret in gateway state (sqlite db, state/data dirs) | not found |
| secret in OpenBao KV `secret/openshell/provider-credentials/<id>` | found |
| OpenBao down: create a credential | refused |
| OpenBao down: create a sandbox using the credential | refused |
| OpenBao down: sandbox already running | still works (see below) |

## How it works

Gateway config only, nothing changes for providers, profiles or sandboxes:

```toml
[openshell.gateway]
credential_drivers = ["vault"]

[openshell.credential_drivers.vault]
address = "http://127.0.0.1:8200"   # plain http only on loopback
mount = "secret"
kv_version = "2"
auth_method = "token_file"          # or "kubernetes"
token_path = "<file with an OpenBao token>"
```

The OpenBao token needs create/read/update/delete/list on `secret/*`.

## Limits

- A running sandbox keeps working while OpenBao is down: the supervisor holds the resolved secret. Stopping OpenBao is not a revocation.
- The gateway's OpenBao token is a file on the gateway host (`token_file`). It replaces the local key file as the thing to protect; it can be short-lived and revoked centrally. `kubernetes` auth is the only other method.
- One credential driver per gateway, no migration: credentials created before the switch stay in the database until recreated.
- PoC ran OpenBao in dev mode (in memory, root token, http on loopback) and a throwaway gateway without TLS/auth. Not covered: OpenBao as a service in `zero install`, unseal, TLS, token renewal.

## Reproduce

```
BAO=<bao binary> IMAGE=<image with curl> pocs/10-openbao/run.sh
```

Needs a rootless podman socket for the calling user (`systemctl --user start podman.socket`) and the pinned gateway binary from `zero install`. The real `openshell` gateway is not touched.
