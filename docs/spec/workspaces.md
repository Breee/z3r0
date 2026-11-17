# Spec: workspaces and contexts

Status: draft (#71). Built: config, context resolution, `zero ctx`, `-c`, `ZERO_CONTEXT`, migration (#75); multi-folder sandbox with repo folders, union policy, one token for all projects (#76). input folders (#77, ADR 0039). Open: `zero ls` + rm check #79.

## 1. Concepts

| Term | Meaning |
|---|---|
| Workspace | One sandbox. Named. Holds 1..n folders. Like a VS Code multi-root workspace. |
| Folder | A host directory in a workspace. Kind `repo` (git) or `input` (no git repo). |
| Context | The workspace a command acts on. kubectl-style. |
| Current context | Stored fallback, set by `zero ctx use`. |

Rules that do not change: agent never sees real secrets; the only exit for work is `git push` (ADR 0027); policy is read from the host, never from the sandbox.

## 2. Files

`~/.config/zero/config.yaml` (written by zero, user may read/edit):

```yaml
current: team
workspaces:
  team:
    folders:
      - /home/bree/src/api
      - /home/bree/src/web
      - /home/bree/notes/specs
  developer:
    folders:
      - /tmp/opencode/developer
```

Runtime state stays per workspace: `~/.local/state/zero/zero-<name>/` (port, forge token id, policy sha, lock). Sandbox name `zero-<name>`.

Names: `[a-z0-9-]{1,40}`, unique. Folder paths stored as resolved absolute paths (symlinks resolved).

## 3. Context resolution

First match wins:

1. `-c NAME` / `--context NAME`
2. `ZERO_CONTEXT`
3. cwd: the workspace whose folder contains the resolved cwd
4. `current` in config.yaml
5. none → commands that need a workspace fail with a hint; `zero up` creates one (§5.1).

Exception (built in #75): `zero up|web|code|attach|run` inside a git repo that belongs to no workspace creates a new workspace for that repo instead of acting on `current`.

Every command that acts on a workspace prints its first line to stderr:
`ctx: team (cwd)` — source is one of `flag`, `env`, `cwd`, `current`, `new`.

cwd ambiguity (folder in several workspaces): if `current` is one of them use it, else fail and list them.

## 4. Commands

```
zero ctx                         list; * = current, + = matches cwd
zero ctx use NAME                set current
zero ctx new NAME [PATH...]      create (default PATH = cwd), set current. No sandbox yet.
zero ctx add [PATH...]           add folders to context (default cwd)
zero ctx remove [PATH...]        remove folders from context
zero ctx rm NAME                 = zero rm + delete config entry
zero ctx --prompt                print name only (for PS1), empty if none

zero up | web | code | status | stop | logs | rm    act on context
zero ls                          all workspaces + live status + orphans
```

`ls` output:

```
NAME        STATUS   FOLDERS  TOKEN      POLICY
* team      running  3        gitlab 20h  a1b2c3
  developer stopped  1        gitlab 6h   9e841a dirty
orphans: token 29481100 zero-old@io (no sandbox)
```

## 5. Examples

### 5.1 First use, single repo (today's flow, unchanged)

```
$ cd ~/src/api
$ zero up
ctx: api (new)
api ready: http://127.0.0.1:31234 dir /sandbox/api
```

No context matched → workspace `api` created from cwd, set current. Same as today.

### 5.2 Multi-repo workspace

```
$ zero ctx new team ~/src/api ~/src/web
$ zero up
ctx: team (current)
policy: api(.zero/policy.yaml@4f1a) + web(default)
gitlab token: projects api, web
team ready: http://127.0.0.1:31240 dir /sandbox
```

Sandbox layout: `/sandbox/api`, `/sandbox/web`. One GitLab token with `project_ids: [api, web]`.

### 5.3 Add an input folder

```
$ zero ctx add ~/notes/specs
$ zero up
ctx: team (current)
input specs: 14 files, 220 KiB, read-only
  excluded: .env (secret pattern)
policy unchanged
```

zero copies the folder, filtered, to `/var/lib/zero/input/<sandbox>/specs` on the host. The sandbox sees it at `/tmp/zero-input/specs` through a read-only bind mount (ADR 0039). Refreshed on every `up`; a folder added later needs no new sandbox. Agent edits impossible → nothing to lose. A sandbox created before #77 has no mount: `zero rm && zero up` once. Podman gateway only.

### 5.4 Add a repo to a running workspace (widening)

```
$ zero ctx add ~/src/infra
$ zero up
ctx: team (current)
policy widens:
  + network registry.terraform.io
  + gitlab project infra (git.push, gitlab.mr.create)
approve? [y/N] y
gitlab token: rotated (api, web, infra)
```

Live: clone, new token (#70 rotate flow), egress added. Non-interactive (`up` from script) refuses widening unless `--approve`.

### 5.5 Remove a repo (narrowing)

```
$ zero ctx remove ~/src/infra
$ zero up
ctx: team (current)
gitlab token: rotated (api, web)
network rules for infra remain until recreate (zero rm && zero up)
/sandbox/infra: unpushed branches: feat/x  → kept in sandbox, not reachable via token
```

Token narrowing is immediate; the push gate refuses the removed repo at once. Egress narrowing needs recreate (OpenShell rules are additive). Built in #76: the checkout of a removed folder stays in the sandbox; the unpushed-branches line is not printed yet (#79). All repo folders of a workspace must be on the same forge host.

### 5.6 cwd wins over current

```
$ zero ctx use team
$ cd /tmp/opencode/developer
$ zero status
ctx: developer (cwd)
```

### 5.7 Destructive command

```
$ cd ~
$ zero rm
ctx: team (current)
unpushed: api feat/login (2 commits), web: none
type workspace name to delete: team
removed team
```

`-y` skips the prompt but still prints unpushed branches.

A sandbox created again starts from the host checkout. If the forge still has `zero/<name>` with commits that are not in it, `zero up` prints a note per repo; a push is rejected until the agent pulls that branch or the user deletes it on the forge (the gate allows no delete). Built in #115.

### 5.8 Ambiguous cwd

```
$ zero ctx new hotfix ~/src/api
$ zero ctx use developer
$ cd ~/src/api && zero status
zero: ~/src/api is in workspaces team, hotfix; use -c or zero ctx use
```

### 5.9 Two branches of one repo in parallel

Use git worktrees on the host: `~/src/api` and `~/src/api-hotfix` are different paths → different workspaces, no ambiguity. Each workspace clones the branch checked out in its folder.

### 5.10 Folder moved or deleted on host

```
$ zero up
ctx: team (current)
zero: folder /home/bree/src/web missing; zero ctx remove it or restore it
```

No silent drop.

### 5.11 Migration from today's state

Existing `~/.local/state/zero/zero-<repo>/` without config entry: the first command run inside that repo registers it as workspace `<repo>` with the repo as folder (state does not record the host path, so it cannot be registered from elsewhere). No sandbox recreate. Verified #75.

## 6. Folder handling

| | repo | input |
|---|---|---|
| Detect | folder has `.git` (without a forge remote: checkout, no token, no push) | no `.git` |
| Into sandbox | git bundle of host commits, current branch checked out | filtered host copy, read-only bind mount at `/tmp/zero-input/<name>` |
| Uncommitted host changes | not included (warn) | included |
| Excluded | gitignored by nature | always (not overridable): `.env*`, `*.pem`, `*.key`, `id_*`, `.git-credentials`, `.netrc`, `.ssh/`, `.aws/`, `.kube/`, symlinks, special files; plus `.zeroignore` (one pattern per line: file name or path relative to the folder, shell wildcards) |
| Exit | `git push` via forge token | none |
| Policy source | `<folder>/.zero/policy.yaml` on host | none (no network grants) |
| Size limit | — | 100 MiB for all input folders together, override `zero up --max-input <MiB>` |

## 7. Policy and credentials

- Workspace policy = union of repo folders' policies (network.allow ∪, capabilities ∪ per project).
- Token permissions are per workspace, not per project (GitLab granular token: one permission set, many project_ids) → union applies to all repos. Recorded in `zero ls` / `status`.
- Widening vs last approved (stored `state/<name>/policy.approved`) → prompt. Narrowing → no prompt.
- One token per forge per workspace. Label `zero-<name>@<host>`.

## 8. Flaws found while writing examples

| # | Flaw | Decision in this spec |
|---|---|---|
| F1 | Hidden current context → wrong `rm` | cwd first, `ctx:` line, type-name confirm (5.7) |
| F2 | Folder in several workspaces | ambiguity error unless current matches (5.8) |
| F3 | Input folders leak host secrets (`.env`) into sandbox | **decided**: secret patterns (`.env*` etc.) never copied, not overridable; extra excludes via `.zeroignore`; file summary on `up` (5.3) |
| F4 | Permission union: capability for repo A applies to repo B | **decided**: accepted, one token per forge per workspace; shown in status |
| F5 | Egress narrowing not live | print "until recreate" (5.5) |
| F6 | Removed repo keeps unpushed work in sandbox | warn; token no longer covers it (5.5) |
| F7 | Uncommitted host changes silently missing in sandbox | warn on `up` |
| F8 | Name collisions in `/sandbox` (`a/api`, `b/api`) | built in #76: error, a workspace needs distinct folder names (no suffix) |
| F9 | Larger blast radius per workspace | only explicitly listed folders; docs |
| F10 | opencode multi-folder: one serve, sessions per directory | **verified 2026-11-16 (#73)**, opencode 2.2.6: one background service (port 4096) ran `opencode run` from two folders; each session has its own `location.directory`, each file landed in its folder. A folder without commits gets `projectID: global`. Not checked: the web UI project switcher |
| F11 | `zero code` multi-root | **verified 2026-11-16 (#74)**, VS Code 1.145.2: a generated `/tmp/zero/zero.code-workspace` with relative folder paths, opened with `code --file-uri vscode-remote://ssh-remote+<host>/tmp/zero/zero.code-workspace`; the git extension in the sandbox opened both repositories. Not checked: the Explorer by eye |
| F12 | Config edited by hand to invalid state | strict parse (KnownFields), clear error |

## 9. Out of scope

TUI picker and web UI (later, on the same config); GitHub forge; syncing input folders back; renaming workspaces.

## 10. Implementation order (issues)

1. config.yaml + context resolution + `zero ctx` + `-c` + migration
2. multi-folder sandbox (repo folders, union policy, multi-project token)
3. input folders (copy, read-only, exclude list)
4. widening approval prompt
5. `zero ls` with orphans; unpushed check on `rm`
6. picker TUI
