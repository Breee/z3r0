# Scenario: Go developer, containerized app, repo on GitLab

End-to-end walkthrough of one developer using zero. Every step names **who enforces** it and its **status today** (`done` = works and verified, `gap` = missing in zero, `blocked` = upstream bug, `planned` = designed, not built). Architecture background: §13 (target scenario), §7/§7a (policy), ADR 0020 (forge tokens), ADR 0024 (forge owns code flow).

## 0. Actors and boundaries

| Actor | Runs where | Can reach |
|---|---|---|
| Developer `dev` | host, login user | own files, glab session, browser |
| zero CLI | host, as `dev` | glab (host session), OpenShell gateway (mTLS client cert) |
| OpenShell gateway + rootless podman | host, user `openshell` | podman storage, credential store |
| Agent (opencode) + its tools | sandbox (E1 podman, or E3 VM) | `/tmp/zero/<repo>`, allowlisted egress only, credential **placeholders** |
| GitLab | gitlab.com | enforces code flow: protected branches, MR, CI, approvals |

The agent never runs on the host and never sees a real token. Code reaches `main` only through GitLab's MR process.

## 1. One-time host setup

| Step | Command | Result | Status |
|---|---|---|---|
| Install | `sudo zero install` | pinned OpenShell 0.1.4 (sha256 in zero, C41) in `/usr/local/lib/zero/`; user `openshell` with subuids + linger; rootless podman + image allowlist; gateway (podman, fail_closed, no bind mounts, telemetry off) on 127.0.0.1:17670; `dev` gets only the client cert | done |
| GitLab login | `glab auth login` | host session; zero mints per-sandbox tokens from it | done (developer's job) |
| Dev image | today: `scripts/build-dev-image.sh` (local, sudo) → `localhost/z3r0-dev:latest` = golang:1.25 + opencode 2.2.6 + glab 1.128.0 + ripgrep (uniget, digest-pinned) | gap: target is CI-built, cosign-signed ghcr.io image pulled by digest (ADR 0021) |
| Check | `zero doctor` | gateway reachable, gateway not run as `dev`, client key 0600 | done |

## 2. One-time GitLab project setup (forge's job, ADR 0024)

zero does not enforce these; `zero doctor` reports them.

| Setting | Value | doctor checks |
|---|---|---|
| Default branch protection | push: No one, merge: Developers+Maintainers, no force push | yes |
| Merge requires pipeline | `only_allow_merge_if_pipeline_succeeds` | yes |
| Discussions resolved, ff merge, squash, delete source branch | recommended | no |
| CODEOWNERS on `.zero/` + approvals | recommended (Premium) | planned |
| CI | `.gitlab-ci.yml`: lint (gofmt/vet), test (`go test -cover`), build (binary + image), Secret-Detection template; workflow rules MR/default/tag | no |
| Image build in CI | rootless buildah/kaniko/ko in CI → push to GitLab registry with CI job token; agents never push images (ADR 0023) | no |

Reference: project `corewire/technology-and-research/developer` has all of the above except CODEOWNERS (MR !2 pipeline green).

## 3. Project policy `.zero/policy.yaml` (planned, §7a)

Read by zero from the host checkout, never from the sandbox. For this case:

```yaml
network:
  allow:
    - proxy.golang.org
    - sum.golang.org
capabilities: [git.read, git.commit, git.push, gitlab.issue.read, gitlab.mr.read, gitlab.mr.create, gitlab.pipeline.read]
```

Status: gap. Today every sandbox gets fixed egress: opencode.ai, models.opencode.ai, gitlab.com. **Consequence: `go mod download` of non-vendored deps fails today.**

## 4. Start: `zero up` (in the repo checkout)

What zero does, in order (`internal/zero/commands.go`):

1. Lock `~/.local/state/zero/zero-<repo>/lock` (one zero command per repo).
2. Detect forge from `origin` (GitLab regex) → project path.
3. Create sandbox `zero-<repo>` from the dev image with the filesystem policy (system dirs read-only; `/tmp`, `/dev/null`, ptys writable; Landlock best effort).
4. Forge setup (`forge.go`, ADR 0020):
   - import provider profile `zero-gitlab` (cred `GITLAB_TOKEN`, `Authorization` header, gitlab.com:443 REST enforce, `allow_encoded_slash` per ADR 0022, binaries git/git-remote-https/curl/glab/opencode);
   - revoke any leftover token; mint a fine-grained token via `glab api`: project-scoped `read_project read_code download_code push_code create_branch create_merge_request read_merge_request read_job read_pipeline` + user-scoped `execute_mcp_tool`, expires next day;
   - create provider `zero-<repo>-gitlab` (token passed via env, never argv); store token id in state.
5. Egress rule `zen` for the opencode binary (real path, C37).
6. Clone: `git bundle` of the host repo → upload → `git clone` into `/tmp/zero/<repo>`; origin set to https + credential helper returning the **placeholder**; host `user.name/email` copied.
7. Write opencode MCP config: GitLab MCP `https://gitlab.com/api/v4/mcp`, `Authorization: Bearer {env:GITLAB_TOKEN}`.
8. Start `opencode serve` on sandbox 127.0.0.1:4096 with a random password (stdin, never argv); SSH forward to a free host port (own session, C40).
9. Print URL. `zero web` opens it with the login.

Status: done (live-verified on `developer`).

## 5. Development loop inside the sandbox

| Agent action | Path | Enforced by | Status |
|---|---|---|---|
| read/edit code | `/tmp/zero/<repo>` | Landlock fs policy | done |
| `gofmt`, `go vet`, `go test ./...` | local, no network | sandbox process | done |
| `go mod download` | proxy.golang.org | egress policy | gap (needs project policy, §3) |
| tests that open sockets on localhost | inside sandbox | — | done |
| malicious test reads host secrets | — | sandbox has no host mounts; gateway runs as `openshell` | done (PoC01/02, blast-radius) |
| malicious test exfiltrates | any non-allowlisted host | egress deny | done (PoC03) |
| exfiltrate via allowed gitlab.com with attacker's own token | gitlab.com | none (no header matcher) | gap (C18, F13) |

## 6. Containers

| Need | How | Isolation | Status |
|---|---|---|---|
| Build image from Go code without Dockerfile | `ko build --push=false` / tarball | E1 | done (F18 step 2) |
| Build from Dockerfile | buildah/podman in sandbox | E1: impossible (no user namespaces, no caps — by design) | blocked: needs E3 VM (F16 ← OpenShell #4014) |
| Hostile Dockerfile | `RUN curl evil`, mount tricks | E3 + egress policy | blocked (F18 step 4) |
| Run built image for tests | podman inside the VM, no host engine, no push | E3 | blocked (F18 step 5) |
| Base images | only allowlisted registries | podman policy.json in sandbox/VM | planned |
| Push image | — | not allowed; CI builds + pushes (ADR 0023) | by design |

Practical today: build the binary and run tests in E1; build and test the real container image **in GitLab CI**.

## 7. Commit, push, MR

| Agent action | Path | Enforced by | Status |
|---|---|---|---|
| `git commit` | local | — (host identity copied) | done |
| `git push origin feat/x` | gitlab.com, placeholder → token substituted by OpenShell | token scope (push_code) | done |
| `git push -o merge_request.create` / `glab mr create` | GitLab | token scope | done |
| `git push origin HEAD:main` | GitLab | **branch protection** (forge) | done — rejected by GitLab |
| push to other projects | GitLab | token project scope | by design, not tested |
| merge MR | GitLab | no merge permission in token + pipeline required | by design, not tested |
| read token value | sandbox env | only placeholder `openshell:resolve:env:…` present | done |

## 8. CI feedback loop

| Agent action | Tool | Status |
|---|---|---|
| pipeline status, jobs | `glab ci list/view`, `glab api …/pipelines` | done |
| job log | `glab ci trace` / `…/jobs/<id>/trace` | done |
| retry/cancel pipeline | — | denied (no scope) |
| MR read/comment via MCP | GitLab MCP in opencode (46 tools) | done (comment needs scope, not granted) |

Loop: edit → `go test` → commit → push branch → CI → read failures → fix → push → green → human reviews and merges.

## 9. Developer interfaces

| UI | Command | Status |
|---|---|---|
| Web | `zero web` | done |
| VS Code Remote-SSH (+ Copilot inside) | `zero code` | done (not re-verified after Go rewrite, C31) |
| opencode TUI attach | `zero attach` | blocked (opencode #47665, C38) |
| Logs | `zero logs [-f]` | done |
| State | `zero status` | done |

## 10. End: `zero rm`

Stop forward → delete sandbox → revoke token (404/401 tolerated) → delete provider → remove state. Anything not pushed is lost (C30). Token also expires by itself after one day; long sessions need renewal (gap, roadmap core gate 4).

## 11. Evidence (planned, §18)

Per sandbox: policy SHA, token id + scope + expiry, every deny, push events. Today: only OpenShell gateway logs (as user `openshell`) and GitLab's own audit (token name `zero-<repo>`).

## 12. Gaps for this scenario, in order

1. Project policy (§3) — without it, only vendored modules work.
2. Token renewal for sessions > 1 day.
3. Signed dev image from ghcr.io.
4. C18: foreign-token exfiltration via gitlab.com (F13).
5. E3 VM for Dockerfile builds and container tests (upstream #4014).
6. Evidence log.
