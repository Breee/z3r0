# F04 — OpenCode ↔ OpenShell integration
Status: todo
Description: Make OpenCode's tools (bash, edit, read, etc.) execute inside OpenShell, not on the host.
Goal: Pick and prototype the integration path; know if a plugin is needed.
Plan:
1. Option A: run OpenCode itself inside the sandbox (TUI attach / `opencode serve` + `--attach`).
2. Option B: OpenCode on host + plugin that routes tool calls into sandbox (check plugin hooks: `tool.execute.before`, custom tools, shell override).
3. Prototype the cheaper viable option; note session/branch context the plugin could show (arch §11.3).
DONE: a full loop (edit → `go test` → commit) runs from OpenCode with all execution inside sandbox.
GOOD: no host-side file/shell tool path remains usable by the agent; LLM endpoint configurable.
Proof: F02 harness triggered via OpenCode prompt → all blocked; host marker file `~/z3r0-canary` unreadable by agent.
Log:
