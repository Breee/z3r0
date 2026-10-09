# F05 — VS Code Copilot Chat ↔ OpenShell integration
Status: done
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
- 2026-10-09: step 2 Option A works: Copilot agent edited, committed (40e8efa), ran probe 45/0 in sandbox. Fixes: /dev/pts policy (C29), enterprise host, .profile PATH (C09). Token in sandbox (C28).
- 2026-10-09: step 3 Option B rejected (D09): extension can add sandbox tools but not disable built-ins. Custom model endpoint (§11.4) → later.
