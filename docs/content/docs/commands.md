---
title: Commands
weight: 2
description: Every zero command with an example.
---

```
zero [-c WORKSPACE] <command>
```

## Which workspace a command acts on

The first match wins:

1. `-c NAME` or `--context NAME`
2. the variable `ZERO_CONTEXT`
3. the workspace that contains the current folder
4. the current workspace, set by `zero switch` or `zero ctx use`

Every command that acts on a workspace prints the workspace and where it came from as its first line:

```
workspace: developer (cwd)
```

`zero up`, `opencode`, `vscode`, `run` and `web` in a git repository that belongs to no workspace create a new workspace for it.

## Sandbox

| Command | What it does |
|---|---|
| `zero up` | creates the sandbox or reuses it, clones the repositories the first time, starts opencode inside |
| `zero up --approve` | accepts a wider `.zero/policy.yaml` without the question |
| `zero up --max-input MiB` | size limit for input folders, instructions and skills (default 100) |
| `zero up --vm` | new sandbox as a MicroVM instead of a container |
| `zero status` | state of the sandbox, token, push gate |
| `zero stop` | ends the port forward; the sandbox keeps running |
| `zero pause` | stops the sandbox and revokes its token; files stay. Not available for `--vm` sandboxes |
| `zero rm [-y]` | deletes the sandbox; lists work that is not pushed and asks for the workspace name |
| `zero logs [-f]` | log of opencode inside the sandbox |

```sh
zero status
```

```
workspace: developer (cwd)
sandbox zero-developer: up
forward: off
gitlab token: corewire/technology-and-research/developer, corewire/technology-and-research/developer2; capabilities: git.read, git.commit, git.push, gitlab.issue.read, gitlab.mr.read, gitlab.pipeline.read
push gate: on (zero/developer and below)
serve: running
```

Other lines you can see there:

| Line | Meaning |
|---|---|
| `sandbox zero-developer: none` and `next: zero up` | the workspace has no sandbox yet |
| `sandbox zero-developer: paused` | stopped by `zero pause`; `zero up` continues |
| `gitlab token: none; zero up makes a new one` | the token was revoked |
| `forge: none, the agent cannot push ...` | no repository of the workspace is on GitLab, or the sandbox was created without a `glab` login for a self-hosted GitLab |

`zero up` ends with `next: zero opencode`. `zero rm` ends with `workspace NAME is kept; to delete it: zero ctx rm NAME`.

## Working with the agent

| Command | What it does |
|---|---|
| `zero opencode` | `up`, then opencode in this terminal; it runs inside the sandbox, in the first repository. Needs a terminal. `--web` opens the browser instead, like `zero web` (`zero attach` still works) |
| `zero run <prompt>` | `up`, then one prompt without the UI; opencode runs inside the sandbox and uses its default model |
| `zero web` | `up`, then the opencode web UI in the browser. Home lists one session of the workspace's first repository under "Recent sessions"; open it to work in that folder. Other repositories: "Add project" with the path `/tmp/zero/<folder>` |
| `zero vscode` | `up`, then VS Code with Remote-SSH on the sandbox (`zero code` still works) |
| `zero copilot [ARG...]` | `up`, then the GitHub Copilot CLI inside the sandbox, in the first repository. Everything after `copilot` goes to the tool, so the workspace option comes before it (`zero -c app copilot`), for example `zero copilot -p "fix the test"`; without a terminal it needs arguments. It uses the Copilot login of the host's opencode and may reach `api.githubcopilot.com` only, so its built-in GitHub tools do not connect. A sandbox made before this command existed has no Copilot CLI: `zero rm`, then `zero up`. In a workspace with `models` (model proxy) it talks to the proxy instead, needs no Copilot login in the sandbox and needs a model: `zero copilot --model github_copilot/claude-haiku-4.5 -p "fix the test"`, or `model:` for the workspace in zero's config |
| `zero claude [ARG...]` | `up`, then Claude Code inside the sandbox, in the first repository. Only in a workspace that uses the [model proxy](../policy/#models-through-the-model-proxy): it talks to the proxy, with no Anthropic login. Name a model: `zero claude --model github_copilot/claude-haiku-4.5 -p "fix the test"`, or `model:` for the workspace in zero's config. Everything after `claude` goes to the tool, so the workspace is `zero -c app claude`. Without a terminal it needs arguments. A sandbox made before needs `zero rm`, then `zero up` |

zero sets no model unless the workspace names one (`model:`, with the [model proxy](../policy/#models-through-the-model-proxy)). Without it opencode starts with its own default, a free model with a daily limit; when the limit is reached it says so instead of answering. `/models` in the UI lists the models and switches to the one you pick; or set `model` in an `opencode.json` of the repository (this is also what `zero run` uses). The GitHub Copilot models are in the list when the host's opencode had a Copilot login at the time the sandbox was created.

```sh
zero run "add a test for the parser and push"
```

## Workspaces

| Command | What it does |
|---|---|
| `zero ui` | prints a link that works once (5 minutes) and opens a read-only page in the browser: every workspace, what its agent may reach (projects, capabilities, hosts, tools, models, instructions, skills), the push gate's decisions, the parts. Served by zero-cp on `127.0.0.1:18097`; no sandbox can reach it; it changes nothing and shows no token or key. With the model proxy on, the proxy's own page is reached through it below `/litellm/ui/`; that page keeps its own login and is the one place there that can change something |
| `zero ls` | all workspaces with status, token and policy; leftovers without a workspace |
| `zero switch NAME` | makes NAME the current workspace |
| `zero switch` | the same, picked from the list (up/down or j/k, enter, q) |
| `zero ctx` | lists workspaces with their folders (`*` current, `+` contains the current folder) |
| `zero ctx use NAME` | same as `zero switch NAME` |
| `zero ctx new NAME [PATH...]` | new workspace with these folders |
| `zero ctx add [PATH...]` | adds folders to the workspace |
| `zero ctx remove [PATH...]` | removes folders from the workspace |
| `zero ctx rm [-y] NAME` | deletes the workspace and its sandbox; also takes a name from a `leftover:` line of `zero ls` |
| `zero ctx --prompt` | prints the name of the workspace a command would act on, for a shell prompt |

```sh
zero ls
```

```
  WORKSPACE  STATUS   FOLDERS  TOKEN       POLICY
  developer  running  2        gitlab 38h  ok
* z3r0       none     1        -           -
legend: * current workspace; TOKEN: forge and time left; POLICY: ok, or changed = the folders' .zero/policy.yaml differs from what runs; zero up applies it or says why not
```

`STATUS` is `running`, `paused`, `none` (no sandbox) or `unknown` (gateway not reachable). `POLICY` is `changed` when the `.zero/policy.yaml` in the folders differs from what the sandbox runs with; the next `zero up` applies it or says why it cannot; `-` when no policy is recorded (no sandbox yet). The legend goes to the error output, so a script reads rows only.

What is left over without a workspace gets a line each, with the command that removes it:

```
leftover: state /home/bree/.local/state/zero/zero-left145 (no workspace); to remove it: zero ctx rm left145
```

A token without a sandbox is revoked by `zero-cp` within 10 minutes.

`zero switch` only changes the current workspace. It never starts or removes a sandbox.

## Host

| Command | What it does |
|---|---|
| `sudo zero install` | sets up the gateway, podman and `zero-cp`; see [Getting started](../getting-started) |
| `zero doctor` | checks the host setup and, inside a repository, the GitLab project settings |
| `zero completion bash` | prints bash completion: `source <(zero completion bash)` |
| `zero version` | prints the commit and date this build was made from; `zero --version` does the same |

A command refuses an option or argument it does not know and exits with 2 before it starts anything:

```
$ zero up --bogus
zero up: unknown option "--bogus" (options: --approve, --vm, --max-input N)
```

A folder whose name starts with `-` is written as `./-name`.

## Files

| Path | Content |
|---|---|
| `~/.config/zero/config.yaml` | workspaces, their folders, tools, instructions, skills and models, the model proxy, the current workspace |
| `~/.local/state/zero/zero-<name>/` | state of one sandbox (token id, capabilities, push gate log `gate.log`) |
| `.zero/policy.yaml` in a repository | [policy](../policy) of the project |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | done; also `zero status` and `zero ls` when there is no sandbox |
| 1 | failed; the message says why and names the next command |
| 2 | wrong command, option or argument; nothing ran |
