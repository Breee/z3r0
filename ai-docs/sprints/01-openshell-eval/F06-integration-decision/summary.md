# Sprint 01 summary — proofs and gaps

| Claim | Proof | Ref |
|---|---|---|
| Rootless Podman sandbox runs Go/git as uid≠0 | pocs/01-openshell-baseline/run.sh PASS | F01 |
| Agent is not root, no caps, no host Podman/Docker | tests/containment/run.sh 46/46 PASS | F02, D05 |
| Egress default deny, per-binary allowlist | tests/attacks/run.sh egress 10 PASS | F08 |
| Real secret never in sandbox; placeholder resolved upstream | pocs/03 5 PASS | F03 |
| Gateway/supervisor not reachable/tamperable | tests/attacks gateway 17 PASS | F08 |
| Breakout limited to user `openshell` | tests/breakout/blast-radius.sh | F09, D06 |
| OpenCode full loop inside sandbox | pocs/04-opencode-in-sandbox/run.sh 5 PASS | F04, D08 |
| Prompt injection contained (except C27) | pocs/04 attack.sh | F04 |
| VS Code Copilot full loop via Remote-SSH | pocs/05-vscode-remote/run.sh + manual | F05, D09 |

Open high gaps: P01 (C20), P02 (C18), P03 residual (C13, C25, C26), P04 (C17, C27). See attack-paths.md.
