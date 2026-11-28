---
title: Getting started
weight: 1
description: Install zero and start the first workspace.
---

## What you need

| Tool | Why |
|---|---|
| Linux on x86_64 with systemd | the installer pins OpenShell binaries for this platform |
| `podman`, `newuidmap`, `newgidmap`, `curl` | Debian/Ubuntu: `apt install podman uidmap passt curl` |
| Docker instead of podman (optional) | see [Runtime](#runtime) |
| `go` 1.26, `make` | to build zero |
| `glab`, logged in to your GitLab host | zero creates the agent's token with it |
| `opencode`, connected to GitHub Copilot | zero copies its Copilot login into the sandbox for the model |

Log in to GitLab with the keyring, otherwise `zero doctor` warns that the login is stored in plain text:

```sh
glab auth login --hostname gitlab.com --use-keyring
```

## Install

```sh
git clone https://github.com/Breee/z3r0 && cd z3r0
make install   # as your user, not with sudo: builds bin/zero and bin/zero-cp, then runs sudo bin/zero install
```

`zero install` can be run again at any time. It does this:

1. downloads the pinned OpenShell release and checks its checksum
2. creates the user `openshell`; the gateway and the container runtime run under it, not under your login
3. sets up the runtime for that user (podman: an image allowlist and its podman socket)
4. starts the gateway and creates `/var/lib/zero/input`, the place input directories are handed over through
5. gives your login the client certificate for the gateway
6. copies `zero` and `zero-cp` to `/usr/local/bin` and starts `zero-cp` as a user service

`zero-cp` is the service that checks every push of the agent and renews its token.

## Runtime

The sandboxes run in containers. Which program runs them is chosen at install:

| Command | Runtime | Note |
|---|---|---|
| `sudo zero install` | podman, rootless, under the user `openshell` | the default |
| `sudo zero install --docker-rootless` | a rootless Docker daemon of the user `openshell` | needs Docker's `dockerd-rootless.sh` and `rootlesskit` |
| `sudo zero install --docker` | the system's Docker daemon | **dangerous**, see below |

Without a flag a later `sudo zero install` keeps the runtime it finds. `zero doctor` shows it.

**`--docker` is dangerous.** The system's Docker daemon runs as root, and whoever may use its socket is root on this machine. With `--docker` the user `openshell` gets that socket. The sandbox itself is as closed as under the other runtimes, but a flaw in the gateway is then a way to root. Use it only where podman and rootless Docker are no option. `zero install` prints this warning and `zero doctor` repeats it.

To change the runtime, remove the sandboxes first (`zero rm` in each workspace), run `sudo zero install` with the new flag, then `zero up`. Sandboxes of the old runtime are not moved.

Differences under Docker: there is no image allowlist (podman's `policy.json` has no Docker counterpart). With rootless Docker older than 29.10 an input directory on a `nosuid` filesystem cannot be mounted.

## Check the setup

```sh
zero doctor
sudo zero doctor   # checks the gateway's credential store only
```

```
zero 0ff5fed54dd0 2026-11-24
skip  credential store: run `sudo zero doctor`
ok    openshell gateway
ok    gateway user
ok    client key
ok    zero-cp
ok    model login
warn  glab login for gitlab.com is stored in plain text: glab auth login --hostname gitlab.com --use-keyring
ok    guardrails corewire/technology-and-research/developer
```

The first line is the version of the build. The `guardrails` line appears inside a repository only: there `zero doctor` also checks the GitLab project. It reports a default branch that allows direct or force push, merging without a green pipeline and CI variables that are not protected. These settings are what keeps a pushed branch from doing damage, so fix them before the agent works on the project.

`model login` fails when the host's opencode has no GitHub Copilot login; the line names the command (`opencode auth login`). When every workspace uses the [model proxy](../providers/), the line is `na` instead. In a repository that is not on GitLab the last line is `na    forge: ...` instead: the setup is fine, a sandbox for that repository cannot push. `zero doctor` ends with exit code 1 when a line says `FAIL`.

## The first workspace

Go to a clone of a GitLab project and start the sandbox:

```sh
cd ~/src/api
zero up
```

zero creates a workspace named after the directory, a sandbox `zero-api`, clones the repository into it and creates the token.

The first time, when the repository has no `.zero/policy.yaml`, zero asks two things and offers defaults: which package hosts the sandbox may reach (found from files like `go.mod` or `package.json`) and what the agent may do (read, commit and push are on; `ci.run` and `gitlab.mr.create` are off). It shows the file, writes it when you say yes and starts. Every option says in one line what it allows. Enter three times keeps the defaults. Started from the workspace screen, the same questions show inside the screen. Without a terminal, or with `--approve`, it asks nothing and uses the default [policy](../policy).

Then open the agent:

```sh
zero opencode   # opencode in this terminal (runs inside the sandbox)
zero web        # opencode in the browser
zero vscode     # VS Code on the sandbox
```

The agent commits in the sandbox and pushes to the branch `zero/api`. Fetch that branch on the host, review it and open the merge request yourself.

`zero` without a command opens the workspace screen: enter opens opencode, `v` VS Code, `u` starts, `s` stops, `r` removes, `n` makes a new workspace, `d` shows the details, `/` filters, `?` lists every key, `q` leaves.

## More than one directory

A workspace can hold several repositories and directories that are not repositories:

```sh
zero ctx new team ~/src/api ~/src/web ~/notes/specs
zero -c team up
```

Repositories are cloned into the sandbox. Other directories are copied in read-only as input (limit 100 MiB, change it with `zero up --max-input MiB`). One token covers all projects of the workspace.

## Stop and remove

```sh
zero pause      # stops the sandbox and revokes the token; zero up continues (not for --vm)
zero rm         # deletes the sandbox; lists work that is not pushed and asks for the name
```

`zero rm` keeps the workspace, so `zero up` makes a new sandbox for it. `zero ctx rm NAME` deletes the workspace too.

## Shell completion

```sh
echo 'source <(zero completion bash)' >> ~/.bashrc
```
