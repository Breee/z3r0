---
status: accepted
date: 2026-11-26
deciders: bree
requirements: [Z-INV-1, Z-INV-2, Z-INV-3, Z-INV-4, Z-INV-6]
---

# 0049 — zero-cp is the one front door; agents and workspaces are defined in one config file

## Context

zero today isolates an agent and gates its pushes. The goal is wider: people define agents and workspaces with models, instructions, skills and tools in one place, and zero applies them with the same security rules. PoC 15 (tools behind agentgateway), PoC 17 and 18 (models, tools and guardrails through LiteLLM) and the evaluations in `ai-docs/tool-eval-litellm.md` and `ai-docs/tool-eval-k8s-agent-runtimes.md` showed what the parts can do. This ADR fixes the shape for the local version (#157). Nothing in it is built except what the core already has.

## Decision

1. **One front door.** `zero-cp` is the only service a sandbox talks to besides the forge. It routes and decides; it does not do the work of the parts behind it. Today it holds the push gate, the token upkeep and the decision log. It gains an API, a model path and a tool path.
2. **Parts stay separate containers and are opt-in.** Models and guardrails: LiteLLM. Tools: agentgateway (ADR 0043). Secrets: OpenBao, else the host keyring. The core runs unchanged with none of them switched on.
3. **One config file, declarative.** `config.yaml` holds `agents` and `workspaces`. A session is one workspace in one sandbox with one or more agents. An agent is a role: model, instructions, skills, tools, capabilities. The harness (opencode, VS Code, Claude Code, Copilot CLI) decides how an agent shows up.
4. **Three faces, one API.** YAML files, the CLI with a terminal UI, and a web frontend all use the zero-cp API. The YAML stays the source of truth; a UI writes the same file.
5. **Each field has one enforcement point outside the sandbox.** Model: model proxy. Tools: tool gateway. Capabilities: forge token and push gate. Network: egress rule. Instructions and skills: a read-only folder.
6. **Layers only narrow.** Operator limits, then shared definitions, then the repository, then the person. A wider setting needs approval, as `.zero/policy.yaml` does today.
7. **No credential in any config file or in the sandbox.**
8. **Presets name the default.** A preset is a named set of the existing layers. First ones: `gitlab-developer` (today's default) and `k8s-reader` (not built, needs #123).
9. **Instructions and skills are plain files.** zero copies them into the sandbox read-only and tells the harness where they are. No part behind the front door is needed for them.

## Consequences

- Build order, each as its own issue: instructions and skills from the config; model path through zero-cp; read-only UI; tool path; editing in the UI.
- zero-cp becomes the single thing to compromise. It must stay small.
- LiteLLM needs Postgres for keys and its UI, and a LiteLLM key is readable in the sandbox until a placeholder is shown to work (PoC 18).
- Open: whether agents with different rights share one sandbox (they would share files, network and token); tool lists per workspace through agentgateway or LiteLLM; shared definitions from a pinned git repository; operator limits file.
- Running agents on Kubernetes is out of scope here; the local version comes first.
