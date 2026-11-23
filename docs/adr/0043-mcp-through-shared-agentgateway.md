---
status: accepted
date: 2026-11-23
deciders: bree
requirements: [Z-INV-1, Z-INV-2, Z-INV-3, Z-INV-4, Z-INV-5, Z-INV-6]
---
# 0043 — MCP tools go through one shared agentgateway that verifies the sandbox's SVID; opt-in addition

## Context
Today zero registers GitLab's MCP endpoint inside the sandbox together with the token (Z-INV-4 not met for MCP). PoC 13, 14 and 15 (#127, #128, #130, `docs/scenarios/mcp-gateway-identity.md`) showed a chain that keeps the token outside: the OpenShell supervisor injects the sandbox's JWT-SVID, agentgateway verifies it, zero-cp returns the workspace's backend credential and tool list per request.

## Decision
- M1: the gateway verifies the SVID itself. No identity provider of our own, no capability claims in the token; zero-cp answers per request.
- M2: one shared gateway for all sessions.
- M3 (GitLab): the MCP server is the instance endpoint `https://<forge host>/api/v4/mcp` of the workspace's forge, self-hosted included. Each workspace gets a default tool list (read tools, written by zero) and can override it in its config. Kubernetes is not decided.
- M6: the GitLab MCP moves out of the sandbox. zero-cp returns the tool list per request (PoC 15 variant B); zero does not rewrite gateway config per workspace.
- This is an addition, not core: sandbox, workspaces, push gate and glab login work without it, and a workspace without it cannot reach the gateway.
- The gateway runs as a container from `ghcr.io/agentgateway/agentgateway` pinned by digest. The podman image policy gets an entry for `ghcr.io/agentgateway` on hosts that enable the addition.
- Not decided: M4 (limits by rules on tool arguments; until then by credential only) and M5 (git traffic stays as it is: push gate, fetch token in the sandbox).

## Consequences
- More parts to run when the addition is on: SPIRE, agentgateway, zero-cp (MG1). zero-cp is asked on every tool call; if it is down, all tools fail closed (MG9).
- The policy entry accepts any image of that registry path without a signature check (P11); the digest pin is what fixes the content.
- GitLab checks one token permission per tool, so zero needs a tool -> permission table to mint a token that fits the list (MG14).
- Open before it can ship: SPIRE node attestation that survives (MG11), SPIRE entries per sandbox (MG3), gateway-to-zero-cp authentication better than a shared key (MG10), a run against a self-hosted instance, opencode as the client.
