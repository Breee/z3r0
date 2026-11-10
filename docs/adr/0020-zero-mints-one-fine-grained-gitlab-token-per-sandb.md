---
status: accepted
date: 2026-10-16
deciders: bree
requirements: [Z-INV-4, Z-P0-6, Z-P0-7]
---

# 20. zero mints one fine-grained GitLab token per sandbox from the user's glab session (`POST /user/personal_access_tokens`, granular scopes on the single project, expires next day, revoked on `zero rm`), stored only as an OpenShell provider credential

## Context and Problem Statement

user decision; no bot user needed.

## Decision Outcome

zero mints one fine-grained GitLab token per sandbox from the user's glab session (`POST /user/personal_access_tokens`, granular scopes on the single project, expires next day, revoked on `zero rm`), stored only as an OpenShell provider credential. The token inherits the user's role (Maintainer can merge green MRs); accepted: actions are accountable to the user. Direct pushes to `main` are blocked by branch protection (push = No one).
