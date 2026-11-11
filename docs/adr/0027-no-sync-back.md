---
status: accepted
date: 2026-11-11
deciders: bree
requirements: [Z-INV-1]
---
# 0027 — No sync-back: git push is the only exit

## Context
C30 (#48): sandbox work lives only in the sandbox. A `zero sync` was planned to bring it to the host.

## Decision
zero has no sync-back. Work leaves the sandbox only via `git push` to the forge. The developer checks out the branch if they want it. Unpushed work is lost on `zero rm`.

## Consequences
- No host write path from the sandbox; agent files reach the host only via an explicit checkout.
- Same model as cloud agents (branch + MR).
