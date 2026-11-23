---
status: accepted
date: 2026-11-23
deciders: bree
requirements: [Z-INV-1, Z-INV-2, Z-INV-5]
---
# 0046 — The sandbox reaches the forge host for the workspace's projects only

## Context

The token profile allowed git, curl, glab and opencode to reach all of the GitLab host (C35, #52). The token itself is limited to the workspace's projects, but the sandbox could read every public project on the host and could send a token of an attacker to any project, snippet, GraphQL or MCP call on it. That is a way out for data (P02 on the forge host) and a way in for planted text (P26). OpenShell 0.1.8 matches method, path and query, not headers, so a foreign token cannot be told from ours.

Probe: `pocs/16-forge-paths`. Test: `tests/attacks/forge.sh`.

## Decision

1. The provider profile of the token (`zero-gitlab-token`) has no endpoint. Profile endpoints add to a sandbox's rules and cannot be narrowed per sandbox.
2. `zero up` writes a rule `forge` into the sandbox's policy, with the token bound to it. Allowed on the forge host, per project of the workspace:
   - `/api/v4/projects/<id>` and `/api/v4/projects/<encoded path>`: GET; below them GET, HEAD, POST, PUT, PATCH
   - `/<path>.git/info/refs` (GET) and `/<path>.git/git-upload-pack` (POST)
   - pushes stay refused; they go through the push gate (ADR 0030)
   Everything else on the host is refused by OpenShell: other projects, user, group and search API, snippets, GraphQL, the MCP endpoint, uploads, web pages, raw files.
3. When the workspace's projects change, `zero up` replaces the rule on the running sandbox, together with the new token.
4. The GitLab MCP endpoint is no longer registered in the sandbox's opencode config. One URL serves all projects and all tools, so a path rule cannot limit it. MCP comes back through the gateway of ADR 0043.

## Consequences

- A sandbox created before this change keeps its old profile and reaches the whole host until `zero rm` + `zero up`; `zero up` says so.
- `glab` works for the project commands tried (`mr list`, `mr view`, `issue list`, `ci list`); its update check and usage ping are refused, which it reports and ignores. Commands that need the user or group API or GraphQL fail.
- `tools` in the workspace config still decides the token's permissions, but the sandbox has no MCP endpoint until the gateway is built in.
- Left open: a foreign token can still be sent to the workspace's own projects (the attacker would need access to them). Git LFS, group-level API and a self-hosted instance are not tested. `DELETE` is refused for the whole host, as before.
- A project is matched by id and by path. A project that is moved or renamed is unreachable until the next `zero up`.
