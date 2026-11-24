# Security review — zero core (2026-11)

Read-only review of `internal/zero` and the two entry points against Z-INV-1..6; no high finding, six concrete gaps below.
`go vet ./...` and `go test -count=1 ./internal/zero/` pass; findings come from reading the code at the cited lines.

| Invariant | verdict | evidence |
|---|---|---|
| Z-INV-1 agent is not a boundary | partly | all checks are host-side (`gate.go`, `policy.go`, egress rules), but the shared gate has no resource limits (R2) and sandbox text reaches the host terminal unescaped (R4) |
| Z-INV-2 capabilities, least privilege | partly | default caps and project-scoped token are narrow (`policy.go:26-38`, `gitlab.go:27-36`), but the "no forge host in network.allow" guard is skipped when no folder has an origin (R3) |
| Z-INV-3 authenticated, authorized, attributable | partly | `gate.log` records every allow/deny per ref (verified on disk), but requests without a valid key are not recorded (R6) |
| Z-INV-4 no long-lived credentials in the agent | holds | real token only in gateway/zero-cp memory, placeholder in `GITLAB_TOKEN`, token passed to `openshell` via env not argv (`openshell.go:167-188`), no token in the state dir (verified `stat`: 0700/0600) |
| Z-INV-5 containment survives supply chain | holds | dev image pinned by digest (`zero.go:261`), installer tarballs sha256-pinned (`install.sh:17-31`); no signature (known C41/C47) |
| Z-INV-6 evidence and revocation | partly | `rm`/`pause` revoke and a 10-min sweep exists, but revocation is silently skipped when forge state is lost (R1) and a narrowed host list is never removed (R7) |

## Findings

### R1 — med — `zero rm` silently skips token revocation when forge state is missing
`internal/zero/forge.go:276-289`, `382-415`; `internal/zero/serve.go:68-107`; `forge.go:226`.
`revokeForge` returns nil if the `forge` file is absent, even when `forge.id` holds a token, and `Rm` then deletes the state dir (`commands.go:564`). The orphan sweep only runs for forges found in live `forge.id` files, so with no forge sandbox left it never runs.
Scenario: `setupForge` writes `forge`, `forge.host`, `forge.id`, `forge.path` from a map (random order, `forge.go:226`); a crash after `Mint`+`forge.id` but before `forge` leaves a valid GitLab token that no command can revoke — `zero rm` reports success and the token stays valid at GitLab until expiry (≤2 days). Impact is limited: the real token never reached the agent unless the sandbox was fully broken.
Invariant: Z-INV-6.
Fix: write `forge`/`forge.host` before minting; in `revokeForge`/`pauseForge`, return an error (keep state) when `forge.id` is set but the forge is unknown; record a pending id before `Mint` so a sweep can revoke it.

### R2 — med — the shared push gate has no body/time/connection limit
`internal/zero/gate.go:302-320`; `internal/zero/serve.go:36-42`.
After the 30 s command-section deadline the read deadline is cleared and the pack is streamed to the forge with no size cap (`r.ContentLength = -1`); the server sets `ReadHeaderTimeout` only, with no `ReadTimeout` and no cap on concurrent receive-pack handlers.
Scenario: the agent opens many connections to `host.openshell.internal:18095` and trickles pkt-lines or a huge pack; zero-cp (one process for all sandboxes, also doing token upkeep) runs out of fds/CPU and pushes of other sandboxes fail. Denial only, no bypass. Not verified: whether the OpenShell supervisor already bounds connections per sandbox.
Invariant: Z-INV-1.
Fix: wrap the body in `http.MaxBytesReader` with a documented limit, set `ReadTimeout`/`WriteTimeout` on the server, and gate `gateHandler` to a small fixed number of concurrent requests.

### R3 — med — forge-host guard in `network.allow` is skipped when a folder has no readable origin
`internal/zero/policy.go:68-74` (guard), `81-88` (`checkForgeHost`), `internal/zero/forge.go:104-119`.
`checkForgeHost` runs only when `folderForge()` returns a forge. If the host repo has no `origin` (removed, or `git` fails), the check is skipped, yet `applyProjectPolicy` still opens every `network.allow` host read-only for all binaries (`policy.go:144-152`).
Scenario: after `git remote remove origin` (or for a repo with no origin) a `.zero/policy.yaml` containing `network: allow: [gitlab.com]` is accepted — the exact entry ADR 0046 says must be refused — and the whole forge host becomes readable. Needs a fresh sandbox or `zero rm && zero up` (an existing forge sandbox fails earlier in `syncForgePaths`), and the human still sees the widening line.
Invariant: Z-INV-2.
Fix: refuse a `network.allow` entry that equals the recorded `forge.host` (state), and refuse the GitLab host of any folder even when `folderForge` returns nil.

### R4 — low — sandbox text is printed to the host terminal without escaping
`internal/zero/ls.go:243-251` (`unpushedIn`), `internal/zero/commands.go:543-547` (`zero rm`), `644-657` (`zero logs`).
Branch names, counts and log lines come from the sandbox and are written raw to the user's tty.
Scenario: a branch named with `\x1b]0;…\x07` or an OSC-8 link lets the agent repaint the terminal, e.g. cover the "type workspace name to delete" prompt; it cannot feed stdin, so the delete still needs the typed name.
Invariant: Z-INV-1.
Fix: strip control bytes (C0, `\x7f`, CSI/OSC sequences) from every string that originates in the sandbox before printing.

### R5 — low — gate identity is a static greeting and the key compare is not constant time
`internal/zero/gate.go:220` (compare), `482-491` (`gateUp`), `internal/zero/serve.go:126` (`ensureServe`).
`gateUp` treats any process answering `zero-cp` on 127.0.0.1:18095 as the gate, and the key is compared with `!=` after a length check.
Scenario: while zero-cp is down another local process binds 18095 and prints `zero-cp`; `zero up` accepts it and pushes from the sandbox go to that process, which receives the request with the real token already substituted by the supervisor for the `host.openshell.internal:18095` endpoint. (How the supervisor reaches host loopback is not verified.)
Invariant: Z-INV-3, Z-INV-6.
Fix: `subtle.ConstantTimeCompare` for the key, and prove ownership of the port with a random one-time token in the state root (mode 0600) instead of a public greeting.

### R6 — low — refused requests without a session leave no trace
`internal/zero/gate.go:274-278` (unknown key), `372-374` (other method/path).
Only pushes with a valid key are written to `gate.log`/the journal; probes with a wrong key, a foreign repo or a non-push method are answered 403 and forgotten.
Scenario: an agent that finds a key from a removed sandbox or tries another project's path produces no evidence, so a review cannot tell probing from a broken client.
Invariant: Z-INV-3.
Fix: log the peer address, the first 4 hex of the key and the refused path/method to the journal (not to the sandbox, which receives only the 403).

### R7 — low — a host removed from `network.allow` stays reachable until the sandbox is recreated (closed, #141)
`internal/zero/commands.go:108-115`, `internal/zero/policy.go:144-152`, `internal/zero/openshell.go:104-119`.
`applyProjectPolicy` adds endpoints (`--add-endpoint`) and is only called when `ch.newHosts` is non-empty; removed hosts are only reported.
Scenario: the human deletes a host from the policy and runs `zero up`; the sandbox keeps the old endpoint rule, and nothing removes it (only `zero rm` + `zero up` does). The code prints that, but the narrowing is not applied.
Invariant: Z-INV-6.
Fix: replace the whole `project` rule on every `up` with `rt.SetRule` (already exists) built from the current host list, including the empty list.

### R8 — low — the policy approval gate has no test
`internal/zero/policy.go:180-216`, `internal/zero/zero_test.go` (only `TestWidened` exists).
The most security-relevant function — widening detection, the no-terminal refusal, `--approve`, the fallback when `policy.approved` is missing — is untested, while the parser and the gate are fuzzed.
Invariant: Z-INV-2.
Fix: table test with a temp state dir: wider policy without approve and without a tty must error; narrower must pass; a corrupt `policy.approved` must error; caps-only vs host-only widening.

## Confirmed known open points

- `ci.run` + `gitlab.mr.create`: a merge request opened through the API runs a changed pipeline definition; the gate never sees API calls (ADR 0044/0045, §28.3a).
- Forge content written by non-members reaches the agent unmarked (#110).
- A foreign token can be sent to the workspace's own projects; OpenShell matches method/path only (C18, ADR 0046).
- Force-push of its own session branch stays possible; the gate allows it on purpose (ADR 0030, C43).
- PUT/PATCH/DELETE are refused on the forge host, so merge requests and issues cannot be edited (ADR 0046).
- The dev image is verified by digest only; podman cannot check the cosign certificate (C47), and `install.sh` has checksums but no signatures (C41).
- Agent-branch code runs with the login user's rights once the human checks it out (C42).
- The login user holds the gateway mTLS client key, so a breakout as that user is full gateway authority (C26).
- Token renewal only reaches new processes; long-running ones keep the old placeholder (C44, C51).

## Checked and found fine

- The gate serves only receive-pack: `GET …/info/refs?service=git-receive-pack` and `POST …/git-receive-pack`; everything else is 403 (`gate.go:288-291, 372-374`).
- Repo allowlist cannot be bypassed: path is cut at the first `.git/`, the op string must be exactly `git-receive-pack`, and `/g/api.git.git/…`, dot segments and encoded slashes fall through (`gate.go:284-287`; `TestGateSeveralRepos`).
- Ref rule: only `refs/heads/zero/<name>` and below, no deletes, no `..` (`gate.go:184-197`); 32 refs, 1024-byte lines, 30 s for the command section (`gate.go:44-48`).
- `ci.skip` is added unless `ci.run`; client push options are refused unless `merge_request.*` with both `mr.create` and `ci.run` (`gate.go:134-171, 331-364`).
- The definition check fails closed: multi-ref, no token, non-sha1, forge error, timeout or an unclear compare all skip the pipeline; a skipped push with options is refused (`gate.go:385-410, 354-359`).
- The gate holds no credential itself; it forwards the supervisor's Authorization header (`gate.go:27-31`).
- No token in argv (`openshell.go:167-188`) and no token written to state; `gate.log` holds refs/shas/decision only (verified on disk).
- Policy is read from host folders only, with `KnownFields`, a host regex, and a forge-host entry refused (`policy.go:42-118`); `mr.create` without `ci.run` is refused (`policy.go:65-67`).
- Widening needs a terminal `y` or `--approve`; no terminal refuses; comparison is against the last approved file, and a corrupt/unreadable file falls back to the default (fail-closed) (`policy.go:180-201`).
- The agent has no writable host mount: repos are cloned from a bundle (`commands.go:330-351`); input folders are filtered, `O_NOFOLLOW`, size-capped and bind-mounted read-only (`input.go:116-214`, `openshell.go:43-52`).
- Every path/branch that reaches a remote shell is `shellQuote`d (`commands.go:337-350, 419-441`; `ls.go:243`); host commands use `exec` with argv (`zero.go:202-220`).
- The gate listens on 127.0.0.1 only (`serve.go:34`; verified `LISTEN 127.0.0.1:18095 … zero-cp`).
- `rm` asks for the workspace name, deletes, then revokes and keeps state on failure (`commands.go:537-568`); `pause` revokes (`commands.go:506-532`); the sweep revokes this host's unreferenced tokens (`forge.go:382-415`).
- State dirs and files are 0700/0600 (`zero.go:120-161`; verified with `stat`).
- `install.sh` pins both tarballs by sha256 before use, uses a dedicated gateway user, `UMask=0077`, and a hardened `zero-cp` unit (`install.sh:17-31, 66-90, 128-148`).
- Tests cover and fuzz the gate parser, ref/option checks, the input filter, forge rules and policy union (`zero_test.go`); `go vet ./...` and `go test ./internal/zero/` pass.
