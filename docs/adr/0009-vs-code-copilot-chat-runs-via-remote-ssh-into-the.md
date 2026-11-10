---
status: accepted
date: 2026-10-09
deciders: bree
requirements: []
---

# 09. VS Code Copilot Chat runs via Remote-SSH into the sandbox (`openshell sandbox connect N --editor vscode`), not via a host extension

## Context and Problem Statement

PoC05 full loop (edit, commit, probe 45/0) in sandbox; a host extension cannot disable built-in read/edit/terminal tools (same gap as PoC04b).

## Decision Outcome

VS Code Copilot Chat runs via Remote-SSH into the sandbox (`openshell sandbox connect N --editor vscode`), not via a host extension.
