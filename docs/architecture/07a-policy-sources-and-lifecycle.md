## 7a. Project policy: source and lifecycle

Status: implemented (`internal/zero/policy.go`). Hosts are read-only egress for any sandbox binary; zero records `HEAD` SHA + dirty flag in state `policy.sha`. Widening (new host or capability compared to state `policy.approved`, or to the default on a new sandbox) asks `approve? [y/N]`; without a terminal `zero up` refuses unless `--approve` (#78). Narrowing never asks. A changed capability set re-mints the token; new hosts apply live; removed hosts stay reachable until `zero rm` + `zero up`.

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
  - gitlab.pipeline.read
```

No file → this default (§13.1 without containers). Unknown keys or capabilities → `zero up` refuses.

### Capabilities and what zero compiles them to

| Capability | OpenShell / token |
|---|---|
| `git.read` | `read_project read_code download_code` |
| `git.commit` | local only (see C42: committed hooks run on host after checkout) |
| `git.push` | `push_code` |
| `gitlab.issue.read` | `read_work_item` |
| `gitlab.mr.read` | `read_merge_request` |
| `gitlab.mr.create` | `create_merge_request`; push options `merge_request.*` pass the gate. Not in the default set; refused without `ci.run` (ADR 0044) |
| `gitlab.pipeline.read` | `read_pipeline read_job` |
| `ci.run` | `read_repository`; the push gate still skips the pipeline while the session branch changes a pipeline definition file (ADR 0045). Without it the push gate adds `ci.skip` to every push (ADR 0035) and refuses every push option of the agent (ADR 0044). Not in the default set |
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
