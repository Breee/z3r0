---
status: accepted
date: 2026-11-10
deciders: bree
requirements: []
---

# 22. GitLab endpoint in profile `zero-gitlab` sets `allow_encoded_slash: true` (C36)

## Context and Problem Statement

user decision; GitLab APIs use `%2F` project paths (glab), endpoint has no path-scoped rules so nothing is bypassed; token scope remains the limit.

## Decision Outcome

GitLab endpoint in profile `zero-gitlab` sets `allow_encoded_slash: true` (C36).
