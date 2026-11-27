# Spec: everyday CLI

Status: draft (#185). PoC 31. ADR 0058. Runtime per workspace: #180. Styled output everywhere: #181.

## 1. What is typed

| Typed | Does |
|---|---|
| `zero` | On a terminal: the workspace list (section 3). Not a terminal: help, exit 2. |
| `zero opencode`, `zero vscode`, `zero claude`, `zero copilot` | Sandbox up, agent open. The everyday command. |
| `zero up` | Prepares the sandbox, opens nothing. For scripts and warm-up. |
| `zero ls`, `zero stop`, `zero rm`, `zero doctor`, ... | Unchanged. |

Rules:

- One spelling per action. There is no `zero up --opencode`.
- The workspace comes from `-c`, else `ZERO_CONTEXT`, else the current directory, else the current context (workspaces spec, section 4).
- A second run asks nothing.

## 2. First run

A first run is: the sandbox does not exist yet and at least one repo directory of the workspace has no `.zero/policy.yaml`.

On a terminal (stdin and stderr), without `--approve`, zero asks once per workspace:

1. **Hosts.** Presets found in the repo directories that have no policy file, selected by default:

   | File | Preset | Hosts |
   |---|---|---|
   | `go.mod` | Go modules | `proxy.golang.org`, `sum.golang.org` |
   | `package.json` | npm | `registry.npmjs.org` |
   | `requirements.txt`, `pyproject.toml` | Python | `pypi.org`, `files.pythonhosted.org` |
   | `Cargo.toml` | Rust | `crates.io`, `index.crates.io`, `static.crates.io` |

   The forge host is never offered (the forge rule covers it). No preset found: the question is skipped.
2. **Permissions.** The default set is selected: `git.read`, `git.commit`, `git.push`, `gitlab.issue.read`, `gitlab.mr.read`, `gitlab.pipeline.read`. Off and explained: `ci.run` (pipelines run for the agent's branch, ADR 0035), `gitlab.mr.create` (needs `ci.run`, ADR 0044).
3. **Confirm.** zero shows the policy of the workspace as it will be and asks to write it.

Yes: `.zero/policy.yaml` is written into each repo directory that had none, and this answer is the approval of the workspace policy that was shown (with several directories: all of them together), and of nothing beyond it. A directory whose own file allows more than what was shown still gets the `approve?` question. No: nothing is written, the default policy applies, the sandbox starts. Esc or Ctrl+C: nothing is written, the command stops.

Enter on every question keeps the defaults.

Not a terminal, or `--approve`: no questions, behaviour as before.

## 3. Workspace list

`zero` on a terminal shows every workspace with status, directories, token and policy state; the current one is selected.

| Key | Does |
|---|---|
| `↑` `↓` `j` `k` | Move |
| `enter`, `o` | `zero -c NAME opencode` |
| `v` | `zero -c NAME vscode` |
| `s` | `zero -c NAME stop`, back to the list |
| `r` | `zero -c NAME rm` (asks as `zero rm` does), back to the list |
| `n` | New workspace: name and directory, then back to the list |
| `q`, `esc`, `ctrl+c` | Quit |

The list runs the same commands a person would type; it has no logic of its own.

## 4. Who owns which config

| File | Owner | Holds |
|---|---|---|
| `REPO/.zero/policy.yaml` | The repo, committed | Hosts and permissions the repo needs |
| `~/.config/zero/config.yaml` | The person | Workspaces (sets of directories), tools, model |

- No credentials in either file.
- The policy of a workspace is the union of its repo directories' policies.
- The repo file is untrusted input: the agent can edit it. A change that widens the policy takes effect only after approval on the host (`zero up` asks; `--approve` for scripts).
