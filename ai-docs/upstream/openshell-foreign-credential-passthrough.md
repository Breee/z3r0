# Feature request (NVIDIA/OpenShell, template: .github/ISSUE_TEMPLATE/feature_request.yml)

**Title:** Bind the credential principal, not only the destination: reject client-supplied credentials on credential-bound endpoints

---

### User Story

As an operator running coding agents in OpenShell sandboxes with provider credentials (LLM, GitLab, GitHub, cloud APIs),
I want the sandbox to only authenticate to an allowed endpoint with the credential I bound to it,
so that a prompt-injected agent cannot use an allowed endpoint to send data to an account the attacker owns.

### Problem Statement

Placeholders protect the operator's secret, and endpoint binding restricts where it may be sent. Nothing restricts **which** credential is used at an allowed endpoint. A request with a plain-text, non-placeholder credential is forwarded unchanged (verified for `Authorization`, `X-Api-Key`, `?api_key=`) to a credential-bound endpoint. Query parameters can be restricted with rule `query` matchers; there is no header matcher.

Tested with OpenShell 0.1.2, rootless Podman driver, Linux.

Minimal reproduction:
```yaml
# profile.yaml (exact file used in our test)
id: repro-echo
display_name: Repro echo
description: Echo service to show foreign credential passthrough
category: other
credentials:
  - name: token
    description: fake secret
    env_vars: [ECHO_TOKEN]
    required: true
    auth_style: bearer
    header_name: authorization
endpoints:
  - host: httpbin.org
    port: 443
    path: /anything
    protocol: rest
    access: read-only
    enforcement: enforce
binaries: [/usr/bin/curl]
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
1. Policy allows an LLM or SaaS API with a bound provider credential and the write paths the agent legitimately needs (e.g. `POST /v1/chat/completions`, or GitLab `POST /api/v4/**`).
2. Prompt injection (repo file, issue, tool/MCP output) instructs the agent to send source code on an allowed path with `Authorization: Bearer <attacker key>`.
3. Egress check passes (host/path/method allowed); credential check passes (no placeholder present, nothing to resolve or deny).
4. The request is accounted to the attacker's account. Where the service stores request content (stored completions, files, snippets, issues), the attacker reads it later. Narrow path rules (e.g. only our project ID) reduce this but are often impractical for LLM APIs.

Without this feature, users must put their own reverse proxy / LLM gateway in front of every credentialed API to strip and re-inject auth, or accept that allowed SaaS endpoints with write paths are an exfiltration channel. Query parameters can be restricted today; headers cannot. "Allowed host" is not "allowed principal".

### Proposed Design

For endpoints that a provider credential is bound to (on by default, opt-out per endpoint):
- A request carrying a credential in the slot the profile defines (header / query / basic auth) that is not a resolvable placeholder is denied with a distinct error (e.g. `foreign_credential`), similar to the existing `credential_unavailable` denial for unresolvable placeholders.
- The profile/policy can list additional credential-bearing slots (e.g. `x-api-key`, `cookie`, `api_key` query) that get the same treatment.
- Optional per-endpoint mode "always inject": whatever the client sends in the slot is replaced by the bound credential.
- Optional general primitive: header matchers in `rules` / `deny_rules` (name + glob), so operators can deny e.g. any `authorization` not matching the placeholder format.
- Denials emit an OCSF event with endpoint and slot name, never the value.

### Acceptance Criteria

- [ ] On a credential-bound endpoint, a request with a non-placeholder value in the bound auth slot is denied; the upstream receives nothing.
- [ ] Placeholder requests keep working unchanged.
- [ ] Operators can declare extra credential slots (header and query) that are enforced the same way.
- [ ] Opt-out per endpoint exists and is visible in the profile/policy.
- [ ] Denials are logged with endpoint + slot, without the credential value.
- [ ] The reproduction above returns a denial for both foreign-key requests.

### Alternatives Considered

- **External reverse proxy / LLM gateway per API** (strip client auth, inject ours): works, but duplicates OpenShell's provider/placeholder model and moves the secret outside OpenShell.
- **Query allowlists only:** closes `?api_key=`, not headers.
- **Deny all endpoints that accept arbitrary keys:** removes the LLM/SaaS endpoints agents need.

### Agent Investigation

- The `credential_unavailable` denial is in `crates/openshell-supervisor-network/src/l7/relay.rs`; placeholder handling also appears in `l7/rest.rs`, `l7/websocket.rs`, `proxy.rs`. We did not analyse feasibility further.
- Policy schema (`docs/how-it-works/policies/schema.mdx`, REST rules): `method`, `path`, `query`; no header matcher.

### Checklist

- [ ] I've reviewed existing issues and the architecture docs
- [x] This is a design proposal, not a "please build this" request
