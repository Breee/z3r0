# Feature request (NVIDIA/OpenShell, template: .github/ISSUE_TEMPLATE/feature_request.yml)

**Title:** Bind the credential principal, not only the destination: reject client-supplied credentials on credential-bound endpoints

---

### User Story

As an operator running coding agents in OpenShell sandboxes with provider credentials (LLM, GitLab, GitHub, cloud APIs),
I want the sandbox to only authenticate to an allowed endpoint with the credential I bound to it,
so that a prompt-injected agent cannot use an allowed endpoint to send data to an account the attacker owns.

### Problem Statement

Placeholders protect the operator's secret, and endpoint binding restricts where it may be sent. Nothing restricts **which** credential is used at an allowed endpoint. A request with a plain-text, non-placeholder credential (in `Authorization`, `X-Api-Key`, `?api_key=`, basic auth, cookie) is forwarded unchanged to a credential-bound endpoint. Network rules match method, path, and query only; there is no header matcher.

Tested with OpenShell 0.1.2, rootless Podman driver, Linux.

Minimal reproduction:
```yaml
# profile.yaml
id: repro-echo
credentials:
  - name: token
    env: ECHO_TOKEN
    auth: { type: bearer }
endpoints:
  - host: httpbin.org
    port: 443
    protocol: rest
    access: read-only
    rules: [ { allow: { method: GET, path: /anything } } ]
binaries: [ /usr/bin/curl ]
```
```bash
openshell profile import -f profile.yaml
openshell provider create --name echo --type repro-echo --credential ECHO_TOKEN=REAL
openshell sandbox create --name repro --from docker.io/library/golang:1.25 --provider echo --detach
# expected: placeholder resolved, upstream sees "Bearer REAL"
openshell sandbox exec -n repro -- sh -c 'curl -s https://httpbin.org/anything -H "Authorization: Bearer $ECHO_TOKEN"'
# expected: forged placeholder → credential_unavailable (works today)
openshell sandbox exec -n repro -- curl -s https://httpbin.org/anything -H 'Authorization: Bearer openshell:resolve:env:v1999_ECHO_TOKEN'
# gap: foreign key → HTTP 200, upstream sees "Bearer ATTACKER-KEY"
openshell sandbox exec -n repro -- curl -s https://httpbin.org/anything -H 'Authorization: Bearer ATTACKER-KEY'
openshell sandbox exec -n repro -- curl -s 'https://httpbin.org/anything?api_key=ATTACKER-KEY' -H 'X-Api-Key: ATTACKER-KEY'
```

### Impact / Why This Matters

Attack path:
1. Policy allows `api.llm-provider.example:443` with a bound provider credential.
2. Prompt injection (repo file, issue, tool/MCP output) instructs the agent to upload source to `/v1/files` with `Authorization: Bearer <attacker key>`.
3. Egress check passes (host/path/method allowed); credential check passes (no placeholder present, nothing to resolve or deny).
4. The data is stored in the attacker's account at a legitimate SaaS; the attacker retrieves it later.

Without this feature, users must put their own reverse proxy / LLM gateway in front of every credentialed API to strip and re-inject auth, or accept that every allowed SaaS endpoint is an exfiltration channel. Query parameters can be allowlisted today; headers cannot. "Allowed host" is not "allowed principal", which defeats the egress control for exactly the endpoints agents need most.

### Proposed Design

For endpoints that a provider credential is bound to (on by default, opt-out per endpoint):
- A request carrying a credential in the slot the profile defines (header / query / basic auth) that is not a resolvable placeholder is denied with a clear error (e.g. `403 foreign_credential`), consistent with the existing `credential_unavailable` behaviour.
- The profile/policy can list additional credential-bearing slots (e.g. `x-api-key`, `cookie`, `api_key` query) that get the same treatment.
- Optional per-endpoint mode "always inject": whatever the client sends in the slot is replaced by the bound credential.
- Optional general primitive: header matchers in `rules` / `deny_rules` (name + glob), so operators can deny e.g. any `authorization` not matching the placeholder format.
- Denials emit an OCSF event with endpoint and slot name, never the value.

### Acceptance Criteria

- [ ] On a credential-bound endpoint, a request with a non-placeholder value in the bound auth slot is denied; the upstream receives nothing.
- [ ] Placeholder requests keep working unchanged.
- [ ] Operators can declare extra credential slots (header and query) that are enforced the same way.
- [ ] Opt-out per endpoint exists and is visible in `openshell policy`/profile output.
- [ ] Denials are logged with endpoint + slot, without the credential value.
- [ ] The reproduction above returns a denial for both foreign-key requests.

### Alternatives Considered

- **External reverse proxy / LLM gateway per API** (strip client auth, inject ours): works, but duplicates OpenShell's provider/placeholder model and moves the secret outside OpenShell.
- **Query allowlists only:** closes `?api_key=`, not headers.
- **Provider-side org restrictions:** do not help; the attacker's key belongs to the attacker's org.
- **Deny all endpoints that accept arbitrary keys:** removes the LLM/SaaS endpoints agents need.

### Agent Investigation

- Placeholder resolution and the `credential_unavailable` denial live in `crates/openshell-supervisor-network/src/l7/relay.rs` (also `l7/rest.rs`, `l7/websocket.rs`, `proxy.rs`). The proxy already parses the auth slots to rewrite placeholders, so detecting a non-placeholder value in the same slot looks like a small extension of the existing path.
- Policy schema (`docs/how-it-works/policies/schema.mdx`, REST rules) supports `method`, `path`, `query`; no header matcher.

### Checklist

- [ ] I've reviewed existing issues and the architecture docs
- [x] This is a design proposal, not a "please build this" request
