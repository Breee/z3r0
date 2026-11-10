---
status: accepted
date: 2026-11-10
deciders: bree
requirements: [Z-P0-8, Z-RULE-8]
---

# 23. Agents never push container images

## Context and Problem Statement

Not recorded (migrated from ai-docs/decisions.md D23).

## Decision Outcome

Agents never push container images. Agents may build images and run them inside the sandbox (dev environment, tests) (image stays in sandbox storage); publishing is CI-only (same as D21). No registry credential is ever bound to a sandbox. F18 step 5 dropped.
