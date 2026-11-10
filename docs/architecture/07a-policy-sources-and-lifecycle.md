## 7a. Project policy: source and lifecycle

Status: implemented (`internal/zero/policy.go`). Hosts are read-only egress for any sandbox binary; zero records `HEAD` SHA + dirty flag in state `policy.sha`. Not yet: approval prompt when a policy widens.

Start small. One file per project, the same shape as §7/§13.1, a fixed small capability set. No layers, bundles or live grants yet.

### File

`.zero/policy.yaml` in the repo:

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
| `git.commit` | local only (see C42: committed hooks run on host after checkout) |
| `git.push` | `push_code create_branch` |
| `gitlab.issue.read` | `read_work_item` |
| `gitlab.mr.read` | `read_merge_request` |
| `gitlab.mr.create` | `create_merge_request` |
| `gitlab.pipeline.read` | `read_pipeline read_job` |
| `container.*` | needs E3 (§6.2), not available yet |
| `network.allow` host | egress rule host:443 read-only (enforce) |

gitlab.com and the model endpoint are always allowed (zero needs them). Which branch may be pushed, merging and approvals stay GitLab's job (ADR 0024).

### Trust

zero reads `.zero/policy.yaml` from the developer's **host checkout**, never from the sandbox. The sandbox copy is ignored, so the agent cannot change its own policy. Agent changes reach the host only when the developer checks out the agent's branch — the developer's decision, like running any other code from it. zero stores the commit SHA (and a dirty flag) in sandbox state.

### Lifecycle

- `zero up` (new sandbox): read, compile, create.
- Policy changed: applied on recreate (`zero rm && zero up`).
- Invalid file: refuse, never fall back to broader.

### Later (not now)

Org ceiling, live grants, deny→policy suggestions, OpenShell prover in CI — only when a real need appears.
