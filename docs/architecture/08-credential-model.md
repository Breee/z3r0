## 8. Credential model

The agent should request **use of authority**, not the secret itself. OpenShell Providers/Provider Profiles are the preferred sandbox-facing mechanism for placeholder credentials and endpoint-bound substitution. OpenBao (or another enterprise credential backend) remains behind that layer.

Bad:

```text
Agent → OpenBao → "give me GitLab token"
```

Preferred:

```text
Agent
  │
  │ "push feature branch"
  ▼
Z3R0 policy / Phantom Proxy (OpenShell supervisor/provider)
  │
  ├─ identity allowed?
  ├─ repository allowed?
  ├─ branch allowed?
  ├─ operation allowed?
  └─ approval satisfied?
       │
       ▼
OpenBao
  │
  └─ short-lived authority
       │
       ▼
OpenShell supervisor/provider path
       │
       ▼
GitLab
```

### Credential binding

Where possible, bind credentials to:

```text
identity
+
destination
+
protocol
+
operation
+
resource
+
TTL
```

Example:

```text
GitLab credential
  valid for:
    host = gitlab.com
    project = acme/payments-api
    operation = repository-write
    TTL = 60 s
```

This prevents credential laundering such as copying a valid GitLab authorization header to `evil.example`.

### 8.1 Phantom Proxy security invariant

The **Phantom Proxy** is the trusted boundary between users/agents and authenticated or sensitive remote systems.

> **Neither developers nor agent sandboxes receive real secrets.**

Real credentials, tokens, passwords, connection-string secrets, and other reusable authentication material stay behind the Phantom Proxy. Humans and agents interact through brokered authority, phantom credentials, or mediated operations.

```text
Developer / Agent
       │
       ▼
Phantom Proxy
  ├─ authenticate actor/session
  ├─ authorize semantic operation
  ├─ obtain/use real credential
  ├─ replace phantom credential
  ├─ call target system
  ├─ sanitize/redact response
  └─ return safe result
       │
       ▼
Target system
```

The Phantom Proxy performs two distinct but related functions:

1. **Credential replacement/brokering** — real authentication material is used only behind the proxy and is never handed to the developer or agent sandbox.
2. **Sensitive-data sanitization** — responses are transformed before crossing the trust boundary so secrets or sensitive subfields are removed while preserving useful diagnostic context.

The trusted transformation order is:

```text
request
  ↓
policy/capability check
  ↓
credential replacement
  ↓
target system
  ↓
raw response
  ↓
structured sanitization/redaction
  ↓
safe response
  ↓
developer / agent sandbox
```

If Z3R0 cannot safely isolate the sensitive portion of a value, the whole value is redacted.

Examples:

```text
postgres://appuser:SuperSecret@postgres-prod-01.internal:5432/payments
→ postgres://appuser:<REDACTED>@postgres-prod-01.internal:5432/payments

Authorization: Bearer eyJ...
→ Authorization: Bearer <REDACTED>
```

The proxy should prefer **minimum necessary disclosure** over hiding an entire object when a safe structured projection is possible.

### 8.2 Remote routing rule

Authenticated remote access must traverse the Phantom Proxy. Public unauthenticated access may go direct, but remains subject to egress policy.

```text
public unauthenticated content
→ direct path allowed by policy

authenticated GitLab / Kubernetes / registry / cloud / database access
→ Phantom Proxy mandatory
```

This intentionally separates **authentication mediation** from general network egress controls. A public endpoint can still be an exfiltration channel, so direct public traffic remains constrained by network policy.

### 8.3 Optional public-content scanner

Organizations may optionally inspect public content before it is exposed to an agent. This scanner is defense-in-depth and is not a hard security boundary.

```text
public fetch
  ↓
egress policy
  ↓
optional trusted scanner
  ├─ prompt-injection indicators
  ├─ suspicious instruction patterns
  ├─ credential-harvesting instructions
  └─ malicious tool-use suggestions
  ↓
agent
```

The scanner is **policy-driven and optional**. Organizations choose which content classes/sources are scanned. Sane defaults may prioritize webpages, issue bodies, READMEs, documentation, MCP/tool responses, and package metadata over simple machine-readable status endpoints.

Where a model is used for scanning, it should be a trusted local/private model managed by the organization. Scanner/model decisions may only make handling stricter; they must never relax deterministic runtime or egress controls.

### CLI helpers

For tools such as `kubectl`, `git`, or `glab`, prefer a credential helper/proxy that is only usable through the authorized execution path.

The agent must not be able to call the helper to obtain a reusable raw credential.

---

