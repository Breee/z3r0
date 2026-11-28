---
status: accepted
date: 2026-11-28
deciders: bree
requirements: [Z-INV-1, Z-INV-2]
---

# 0060 — Provider interfaces by requirement; packages by component

## Context

`internal/zero` is one package of 30 files. The sandbox system (`Runtime`) and
the forge (`Forge`) have an interface, both shaped by their one
implementation. The model proxy (LiteLLM), tools, instructions, skills and the
agents are called directly. The user wants every part replaceable, with the
interfaces defined from requirements first (#190).

## Decision

1. Every replaceable part has one interface in `internal/core`:
   `SandboxProvider`, `Forge`, `ModelProvider`, `ToolProvider`,
   `InstructionsProvider`, `SkillProvider`, `Agent`. Their text is
   `docs/spec/interfaces.md`.
2. Interfaces are small; optional abilities are separate interfaces.
3. Secrets cross an interface only as `Credential`, towards zero. Only zero
   binds them; the sandbox sees a placeholder.
4. Providers declare what the sandbox must reach as `Egress`; zero makes the
   rules.
5. Providers do not import each other; zero's workflow package connects them.
6. Code is laid out by component (spec, section 3); one component is moved per
   commit with the tests green.
7. No plugin loading and no new implementation in this work.

## Consequences

- A new forge, model source or agent is one package and one registration.
- OpenShell's formats (rule maps, provider profiles) leave zero's workflow code.
- The move touches every file; behaviour, output and exit codes stay.
- Each interface was checked against one second case on paper only; the first
  real second implementation (#186) may still change it.

## Accepted

The user read the spec on 2026-11-28 and decided its open points: one model
provider per workspace; instructions and skills as two interfaces; permission
names depend on what the forge offers (spec, section 6).
