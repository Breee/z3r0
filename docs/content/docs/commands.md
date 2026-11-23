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
ctx: developer (cwd)
```

`zero up`, `attach`, `run`, `code` and `web` in a git repository that belongs to no workspace create a new workspace for it.

## Sandbox

| Command | What it does |
|---|---|
| `zero up` | creates the sandbox or reuses it, clones the repositories the first time, starts opencode inside |
| `zero up --approve` | accepts a wider `.zero/policy.yaml` without the question |
| `zero up --max-input MiB` | size limit for input folders (default 100) |
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
ctx: developer (cwd)
sandbox zero-developer: up
forward: off
gitlab token: corewire/technology-and-research/developer, corewire/technology-and-research/developer2; capabilities: git.read, git.commit, git.push, gitlab.issue.read, gitlab.mr.read, gitlab.pipeline.read
push gate: on (zero/developer and below)
serve: stopped
```

## Working with the agent

| Command | What it does |
|---|---|
| `zero attach` | `up`, then opencode in the terminal, connected to the sandbox |
| `zero run <prompt>` | `up`, then one prompt without the UI |
| `zero web` | `up`, then the opencode web UI in the browser |
| `zero code` | `up`, then VS Code with Remote-SSH on the sandbox |

```sh
zero run "add a test for the parser and push"
```

## Workspaces

| Command | What it does |
|---|---|
| `zero ls` | all workspaces with status, token and policy; leftovers without a workspace |
| `zero switch NAME` | makes NAME the current workspace |
| `zero switch` | the same, picked from the list (up/down or j/k, enter, q) |
| `zero ctx` | lists workspaces with their folders (`*` current, `+` contains the current folder) |
| `zero ctx use NAME` | same as `zero switch NAME` |
| `zero ctx new NAME [PATH...]` | new workspace with these folders |
| `zero ctx add [PATH...]` | adds folders to the workspace |
| `zero ctx remove [PATH...]` | removes folders from the workspace |
| `zero ctx rm [-y] NAME` | deletes the workspace and its sandbox |
| `zero ctx --prompt` | prints the name of the workspace a command would act on, for a shell prompt |

```sh
zero ls
```

```
  NAME       STATUS   FOLDERS  TOKEN       POLICY
* developer  running  2        gitlab 27h  9e841a
```

`TOKEN` is the time until the token expires, `POLICY` the commit the policy was read from.

`zero switch` only changes the current workspace. It never starts or removes a sandbox.

## Host

| Command | What it does |
|---|---|
| `sudo zero install` | sets up the gateway, podman and `zero-cp`; see [Getting started](../getting-started) |
| `zero doctor` | checks the host setup and, inside a repository, the GitLab project settings |
| `zero completion bash` | prints bash completion: `source <(zero completion bash)` |

## Files

| Path | Content |
|---|---|
| `~/.config/zero/config.yaml` | workspaces, their folders and tools, the current workspace |
| `~/.local/state/zero/zero-<name>/` | state of one sandbox (token id, capabilities, push gate log `gate.log`) |
| `.zero/policy.yaml` in a repository | [policy](../policy) of the project |
