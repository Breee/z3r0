---
status: accepted
date: 2026-11-11
deciders: bree
requirements: [Z-INV-1]
---
# 0026 — glab sends the GitLab token as Bearer (GLAB_IS_OAUTH2)

## Context
OpenShell 0.1.8 substitutes a credential only in its declared placement. The `zero-gitlab` profile declares `Authorization` (git, curl, GitLab MCP). glab sends personal access tokens in `PRIVATE-TOKEN`, so every glab API call fails with `credential_placement_mismatch` (C46). One credential has one placement; switching to `PRIVATE-TOKEN` breaks git, curl and MCP (tested).

## Decision
zero sets the non-secret sandbox env `GLAB_IS_OAUTH2=true` for GitLab sandboxes (`Forge.Env`). glab then sends `Authorization: Bearer <placeholder>`, which GitLab accepts for PATs.

## Consequences
- glab, git, curl and MCP all work; the agent still only sees the placeholder.
- Depends on glab's OAuth code path staying compatible with a PAT; breaks silently if glab adds OAuth-only behaviour (refresh). Revisit when OpenShell supports multiple placements per credential.
