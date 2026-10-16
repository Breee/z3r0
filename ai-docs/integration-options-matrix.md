# Integration Options Matrix

How each agent client can run on OpenShell, with PoC ref and decision.

| Client | Option | How | PoC | Decision | Status |
|---|---|---|---|---|---|
| OpenCode | A: inside sandbox | `opencode serve` inside sandbox; host TUI attaches via SSH tunnel | PoC04, zero up/attach (F17) | D08 | chosen |
| OpenCode | B: host + plugin | host runs opencode; plugin overrides `bash` → sandbox; other tools on host | PoC04b | rejected | read/edit/MCP/LSP stay on host |
| VS Code Copilot | A: Remote-SSH | `openshell sandbox connect N --editor vscode`; Copilot runs in remote extension host inside sandbox | PoC05 | D09 | chosen |
| VS Code Copilot | B: host extension | extension adds sandbox tools; built-in read/edit/terminal cannot be disabled | F05 step 3 | rejected | same gap as PoC04b |
| Copilot CLI | — | not evaluated | — | — | concept only (phases.md Phase 1 scope); not in arch |
| Claude Code | — | not evaluated | — | — | concept only; arch §22.7 lists it as a client; same model as D08 (agent inside sandbox) expected |
| Codex | — | not evaluated | — | — | concept only (phases.md Phase 1 scope); not in arch |
