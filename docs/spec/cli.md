# Spec: everyday CLI

Status: draft (#185). PoC 31. ADR 0058. Runtime per workspace: #180. Styled output everywhere: #181.

## 1. What is typed

| Typed | Does |
|---|---|
| `zero` | On a terminal: the workspace screen (section 3). Not a terminal: help, exit 2. |
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

Every option carries one line that says what it allows (the same text in the terminal form, in the workspace screen and in the guide's policy page).

From the workspace screen (section 3) the questions are a view of the screen, titled `first run(NAME)`: one list of checkboxes (hosts, then permissions), `space` toggles, `enter` shows the files that will be written, `enter` again writes them and starts, `esc` goes back a step and writes nothing. The top block is hidden so that every option fits on an 80x24 window; smaller windows scroll. The yes is handed to the started command once (state file `policy.shown`, on the host) and covers only the policy shown.

The terminal form sets the height of each question itself (caveat C63).

## 3. Workspace screen

`zero` on a terminal opens a full-screen view in the style of k9s (ADR 0059). Without a terminal it prints the help and exits with 2.

- Top: the current workspace, runtime, gateway state and zero's version; the keys; the zero logo. Narrow windows drop the keys first (below 78 columns), then the logo (below 48), then the whole top (below 16 rows).
- Middle: a framed table `NAME STATUS DIRS TOKEN POLICY`, titled `workspaces(all)[N]` or `workspaces(/FILTER)[N]`. The current workspace is marked `*` and selected at the start.
- Bottom: one line with the result of the last action, or a warning.

The table loads in the background and reloads every 5 seconds and after each action; the selection stays on the same workspace.

| Key | Does |
|---|---|
| `↑` `↓` `j` `k`, `g` `G` | Move; first, last |
| `enter`, `o` | Leaves the screen and runs `zero -c NAME opencode` |
| `v` | Leaves the screen and runs `zero -c NAME vscode` |
| `u` | `zero -c NAME up`, back to the screen |
| `enter`, `o`, `v`, `u` on a new workspace | A workspace without a sandbox whose repo directory has no policy file: the first run (section 2) opens first |
| `s` | `zero -c NAME stop`, back to the screen |
| `r` | `zero -c NAME rm` (asks as `zero rm` does), back to the screen |
| `n` | New workspace: name and directory, then back to the screen |
| `d` | Describe: status, token, policy, model, directories, permissions, allowed hosts, tools, models |
| `/` | Filter by name; `esc` clears it |
| `:` | Command: `q`, `help`, `all`, `new`, or a workspace name to jump to |
| `?` | Help |
| `w` | Switch to the workspace: it becomes the current one (`zero switch NAME`) |
| `ctrl-r` | Reload now |
| `q`, `ctrl+c` | Quit |
| `esc` | Leaves a view or a prompt; on the table it clears the filter, it does not quit |

The screen runs the same commands a person would type; it has no logic of its own. A failed command shows its last line at the bottom.

## 4. Who owns which config

| File | Owner | Holds |
|---|---|---|
| `REPO/.zero/policy.yaml` | The repo, committed | Hosts and permissions the repo needs |
| `~/.config/zero/config.yaml` | The person | Workspaces (sets of directories), tools, model |

- No credentials in either file.
- The policy of a workspace is the union of its repo directories' policies.
- The repo file is untrusted input: the agent can edit it. A change that widens the policy takes effect only after approval on the host (`zero up` asks; `--approve` for scripts).
