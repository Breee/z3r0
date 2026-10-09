# F05 — VS Code Copilot Chat ↔ OpenShell integration
Status: doing
Description: Make Copilot Chat agent-mode tools (terminal, file edits, MCP) execute inside OpenShell.
Goal: Pick and prototype the integration path; know if an extension is needed.
Plan:
1. Option A: VS Code Remote (SSH/Dev Containers-style) into the sandbox so the extension host runs inside.
2. Option B: extension/MCP server exposing sandbox-backed tools; check if built-in host tools can be disabled.
3. Check custom model endpoint support (arch §11.4).
DONE: Copilot agent-mode loop (edit → test → commit) runs with execution inside sandbox.
GOOD: built-in host terminal/file tools not reachable by agent; documented limits.
Proof: same canary + F02 harness via Copilot Chat prompt → all blocked.
Log:
- 2026-10-09: step 1 SSH plumbing via `sandbox ssh-config`; Copilot hosts allowed only for /sandbox/.vscode-server/**; curl + server download denied. GUI step waits on user.
