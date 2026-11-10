## 7a. Project policy: source and lifecycle

Start small. One file per project, the same shape as §7/§13.1, a fixed small capability set. No layers, bundles or live grants yet.

### File

`.zero/policy.yaml` on the default branch:

```yaml
network:
  allow:
    - proxy.golang.org
    - sum.golang.org

capabilities:
  - git.read
  - git.commit
  - git.push
  - gitlab.issue.read
  - gitlab.mr.read
  - gitlab.mr.create
  - gitlab.pipeline.read
```

No file → this default (§13.1 without containers). Unknown keys or capabilities → `zero up` refuses.

### Capabilities and what zero compiles them to

| Capability | OpenShell / token |
|---|---|
| `git.read` | `read_project read_code download_code` |
| `git.commit` | local only |
| `git.push` | `push_code create_branch` |
| `gitlab.issue.read` | `read_issue` (name unverified) |
| `gitlab.mr.read` | `read_merge_request` |
| `gitlab.mr.create` | `create_merge_request` |
| `gitlab.pipeline.read` | `read_pipeline read_job` |
| `container.*` | needs E3 (§6.2), not available yet |
| `network.allow` host | egress rule host:443 read-only (enforce) |

gitlab.com and the model endpoint are always allowed (zero needs them). Which branch may be pushed, merging and approvals stay GitLab's job (ADR 0024).

### Trust

The agent can edit any file in its sandbox and branches, so zero reads `.zero/policy.yaml` **on the host from the default branch** via the forge API — never from the sandbox or working tree — and stores the commit SHA in sandbox state. Changing the policy = normal MR.

### Lifecycle

- `zero up` (new sandbox): read, compile, create.
- Policy changed on the default branch: applied on recreate (`zero rm && zero up`).
- Invalid file or forge API unreachable: refuse, never fall back to broader.

### Later (not now)

Org ceiling, live grants, deny→policy suggestions, OpenShell prover in CI — only when a real need appears.
