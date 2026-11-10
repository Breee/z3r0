---
status: accepted
date: 2026-10-15
deciders: bree
requirements: []
---

# 12. `zero` CLI is Go (stdlib only, `cmd/zero`, `internal/zero`)

## Context and Problem Statement

shell version had no locking, silent failures, password in argv; Go is the project language (arch §25 Phase 1 "Go developer laptop").

## Decision Outcome

`zero` CLI is Go (stdlib only, `cmd/zero`, `internal/zero`); `scripts/zero` is a build shim.
