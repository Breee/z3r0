---
status: accepted
date: 2026-10-09
deciders: bree
requirements: []
---

# 08. OpenCode runs inside the sandbox (Option A)

## Context and Problem Statement

F04 PoC04 full loop works; host plugin (PoC04b) leaves every non-overridden tool, MCP, LSP on host. Needs own dev image (golang + git + opencode); official opencode image is Alpine without git/go.

## Decision Outcome

OpenCode runs inside the sandbox (Option A); no host plugin for tool routing.
