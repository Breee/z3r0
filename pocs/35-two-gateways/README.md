# PoC 35: two gateways side by side, one per runtime

Issue #180.

## Question

Can two OpenShell gateways, one per runtime, run side by side on one machine,
each hold its own sandboxes, and can the gateway be picked per sandbox?
That is what a runtime per workspace needs (`zero opencode --podman`,
`zero opencode --rootless-docker`).

## Result (2026-11-28, OpenShell 0.1.8, podman with crun, Docker 29.5.2 rootless)

Yes. 19 PASS, `fails: 0`.

| Check | Result |
|---|---|
| Gateway A uses the podman driver, gateway B the Docker driver | PASS |
| The installed gateway stays the active one | PASS |
| Both sandboxes are ready at the same time | PASS |
| Each gateway lists its own sandbox and not the other's | PASS |
| The installed gateway knows neither | PASS |
| A's sandbox runs in podman, B's in Docker, none in the other engine | PASS |
| The agent is not root in both (uid 1000) | PASS |
| A host outside the policy is refused in both | PASS |
| `gateway info` names the driver of each gateway | PASS |

## Findings

- `openshell --gateway NAME` picks the gateway for one call. Nothing else is
  shared: each gateway has its own port, config, state, certificates and
  registration.
- The podman driver makes one container per sandbox, the Docker driver two
  (workload and supervisor).
- `openshell gateway add` makes the new gateway the active one and writes its
  own certificates. An install has to select the default gateway again and
  copy the client certificates afterwards.
- The image allowlist of podman (`policy.json`) belongs to the user the
  gateway runs as. The login user's allowlist refused zero's image, so the
  podman side ran OpenShell's default image. Under the `openshell` user zero
  writes the allowlist itself.
- zero already follows the gateway: a sandbox has a gateway name, and the
  mount settings are sent under the name of that gateway's driver.

## Sources

- `openshell-gateway --help` (`--port`, `--config`, `OPENSHELL_*`).
- OpenShell v0.1.8 `docs/how-it-works/sandboxes/runtimes.mdx`,
  `crates/openshell-driver-docker/README.md`.
- PoC 29 (Docker driver), PoC 30 (gateway as user `openshell`), PoC 34 (gVisor).

## Limits

- Both gateways ran as the login user, started by hand. Not shown: two
  installed gateways as units of user `openshell` (needs `sudo zero install`).
- zero's own flow (`zero up` on the second gateway, forge rule, push gate,
  attack suite) was not run.
- The ports were chosen by hand (17690, 17691).
- The podman side used OpenShell's default image, which has no curl. The
  refused host was checked there with a plain TCP connect from bash, a weaker
  check than the HTTP request used on the Docker side.
- No input mount in this PoC (shown for Docker in PoC 29).

## Run

```
pocs/35-two-gateways/run.sh
```

Needs the calling user's `podman.socket` and rootless Docker. Do not run it
together with PoC 29 or 34 (same port).
