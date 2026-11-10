## 7a. Policy sources and lifecycle

§7 defines *what* policy expresses and *where* it is enforced. This section defines *who* authors it, *where* it lives, *how* it changes and *how* zero proves what was in force. Principle (ADR 0024): zero owns the sandbox boundary; the forge owns code flow (branch protection, approvals, CI). zero never re-implements forge controls, it only reports them (`zero doctor`).

### Sources

| # | Source | Owner | Stored | Changes via | Lifetime |
|---|---|---|---|---|---|
| 1 | **Floor** | zero | compiled into zero | zero release | always |
| 2 | **Ceiling** (org baseline) | platform team | signed file / central service (later) | org review | until replaced |
| 3 | **Project policy** | repo owners | `.zero/policy.yaml` on the default branch | MR on the forge | until next change |
| 4 | **Session grant** | developer (human) | sandbox state + evidence | explicit approval | TTL, one sandbox |

- **Floor** = sandbox-boundary invariants nobody can lower: system paths read-only, no host mounts, no wildcard egress, no raw credentials in the sandbox (placeholders only), tokens short-lived and scoped to one project, fail closed. Code-flow rules (who may push/merge) are *not* part of the floor.
- **Ceiling** = the maximum any project may request (allowed hosts/capability bundles, max token scope, required isolation per risk class). Absent ceiling = floor-only default (laptop single-user).
- **Project policy** = what this repo needs beyond the default (extra egress, smaller token scope, isolation class).
- **Session grant** = a temporary, narrow addition while working; never written back automatically.

Effective policy: `floor ∧ (project ∪ grants) ∩ ceiling`. Anything above the ceiling or below the floor is rejected, never clipped silently.

### Trust: where zero reads project policy from

The agent can write every file in its sandbox and in its branches. Therefore:

- zero reads `.zero/policy.yaml` **on the host** via the forge API at the default branch HEAD — never from the sandbox or the working tree.
- The commit SHA of the policy in force is recorded in sandbox state and evidence.
- A policy change is a normal code change: MR → CI → review → merge. Who must approve (e.g. CODEOWNERS on `.zero/`) is the forge's job; `zero doctor` may report it.
- An agent branch that edits `.zero/policy.yaml` has no effect until merged.

### Schema (semantic, not raw)

Project policy names capabilities (§5) and bundles, not OpenShell rules:

```yaml
version: 1
isolation: e1            # e1 podman | e3 vm (§6.2)
bundles: [go-modules]    # zero-maintained: go-modules → proxy.golang.org, sum.golang.org for binary go
egress:
  - host: registry.example.com
    binaries: [go]
    access: read
forge:
  scopes: [read_code, push_code, create_merge_request, read_pipeline, read_job]   # subset of zero default
```

Bundles keep projects from hand-writing host lists and let zero fix hosts centrally.

### Compilation

`zero policy render` deterministically compiles the effective policy to the runtime (`Runtime` interface, ADR 0024):

| Field | OpenShell target |
|---|---|
| filesystem | sandbox create `--policy` (floor only) |
| egress, bundles | network rules per binary (`policy update`) |
| forge.scopes | token mint scope + provider profile |
| isolation | gateway selection (ADR 0015) |

The same function backs `zero policy check` (CI job on MRs touching `.zero/`) so reviewers see the rendered delta, not only YAML.

### Lifecycle

1. `zero up` (create): read policy at default-branch HEAD, render, create sandbox, record SHA.
2. `zero up` (existing sandbox): if the default-branch SHA changed, additive changes apply live; removals require recreating the sandbox (until runtime support for live narrowing is verified).
3. Denial: the agent hits a deny → OpenShell logs it (OCSF) → `zero policy suggest` turns denials into a minimal policy diff.
4. The developer chooses: **session grant** (live, TTL, this sandbox, evidence entry) or **MR** to `.zero/policy.yaml` (permanent, reviewed).
5. `zero rm` drops all grants with the sandbox.

### Proof

- CI: `zero policy check` = schema + floor + ceiling. Later the OpenShell prover (`openshell-prover`, 0.1.4) validates the rendered OpenShell policy (Z-OS-10, unvalidated).
- Runtime: evidence records policy SHA, rendered digest, every grant (who, what, TTL) and every deny.

### Failure behaviour (fail closed)

- Policy file missing → floor-only default.
- Policy unreadable, invalid, above the ceiling, or forge API unreachable → `zero up` refuses; no fallback to a broader policy.
- Grant expiry or `zero rm` → removed; an unknown runtime state → recreate, not guess.

### Open questions

- Does OpenShell `policy update` support removing rules live, or only merging? (decides step 2)
- Ceiling distribution on a laptop before a central service exists (signed file in zero config?).
- Bundle catalogue: where maintained, how versioned.
- Grant approval UX: CLI prompt, `zero web`, or Paperclip (ADR 0017).
