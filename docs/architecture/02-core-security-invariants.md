## 2. Core security invariants

### Z1 — The agent process is not trusted as a security boundary [Z-INV-1]

The agent may be benign, but once corrupted it must be treated like arbitrary code operating with the authority of its current session.

The agent must not be able to disable or bypass:

- filesystem isolation;
- network policy;
- credential mediation;
- capability policy;
- audit logging;
- approval requirements.

Enforcement therefore lives **outside the agent-controlled process tree**.

### Z2 — Capabilities, not ambient authority [Z-INV-2]

The agent receives explicit capabilities for specific operations and resources.

Bad:

```text
glab = allowed
kubectl = allowed
docker = allowed
```

Good:

```text
gitlab.pipeline.read
k8s.pod.logs
k8s.workload.restart
container.build
container.run
```

The CLI is an interface. The **operation** is the capability.

### Z3 — Every consequential request is authenticated, authorized, and attributable [Z-INV-3]

Every remote or side-effecting action should answer:

```text
WHO is acting?
        ↓
WHAT capability is requested?
        ↓
ON which resource?
        ↓
UNDER which constraints?
        ↓
DOES it require approval?
        ↓
HOW is authority delivered?
        ↓
WHAT evidence is recorded?
```

Authorization is per operation and resource, not “the session is trusted now”.

### Z4 — Agents do not receive long-lived credentials [Z-INV-4]

Long-lived credentials must never be placed into agent-controlled:

- environment variables;
- files;
- argv;
- shell history;
- config directories;
- logs.

Prefer a **credential boundary** where the agent receives only a placeholder/phantom credential and the real credential is injected at the outbound boundary after policy evaluation.

Where proxying is not possible, use a narrowly scoped, short-lived credential minted just in time.

### Z5 — Runtime containment must survive supply-chain compromise [Z-INV-5]

DevOps controls reduce the chance that malicious code reaches the agent, but runtime security must assume those controls can fail.

A malicious test, dependency, Dockerfile, build script, CLI plugin, or binary must not automatically gain access to:

- host secrets;
- arbitrary host files;
- unrestricted network egress;
- production credentials;
- unrelated repositories;
- the developer's normal container daemon.

### Z6 — Evidence and revocation are part of authorization [Z-INV-6]

Every significant action must be correlated to a session/trace identity.

A human or automation must be able to revoke a session or capability remotely without relying on the workstation to cooperate.

---

