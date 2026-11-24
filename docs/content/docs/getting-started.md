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
| `go` 1.26, `make` | to build zero |
| `glab`, logged in to your GitLab host | zero creates the agent's token with it |
| `opencode`, connected to GitHub Copilot | the terminal UI on the host; the sandbox uses its Copilot login for the model |

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
2. creates the user `openshell`; the gateway and rootless podman run under it, not under your login
3. sets a podman image allowlist for that user and enables its podman socket
4. starts the gateway and creates `/var/lib/zero/input`, the place input folders are handed over through
5. gives your login the client certificate for the gateway
6. copies `zero` and `zero-cp` to `/usr/local/bin` and starts `zero-cp` as a user service

`zero-cp` is the service that checks every push of the agent and renews its token.

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

`model login` fails when the host's opencode has no GitHub Copilot login; the line names the command (`opencode auth login`). In a repository that is not on GitLab the last line is `na    forge: ...` instead: the setup is fine, a sandbox for that repository cannot push. `zero doctor` ends with exit code 1 when a line says `FAIL`.

## The first workspace

Go to a clone of a GitLab project and start the sandbox:

```sh
cd ~/src/api
zero up
```

zero creates a workspace named after the folder, a sandbox `zero-api`, clones the repository into it and creates the token. Then open the agent:

```sh
zero opencode   # opencode in the terminal
zero web        # opencode in the browser
zero vscode     # VS Code on the sandbox
```

The agent commits in the sandbox and pushes to the branch `zero/api`. Fetch that branch on the host, review it and open the merge request yourself.

## More than one folder

A workspace can hold several repositories and folders that are not repositories:

```sh
zero ctx new team ~/src/api ~/src/web ~/notes/specs
zero -c team up
```

Repositories are cloned into the sandbox. Other folders are copied in read-only as input (limit 100 MiB, change it with `zero up --max-input MiB`). One token covers all projects of the workspace.

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
