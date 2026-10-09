# PoCs

Executable proofs per feature. Each dir: `run.sh` (exits 0 = PASS, cleans up after itself).

| PoC | Feature | Run |
|---|---|---|
| 01-openshell-baseline | F01: rootless sandbox, non-root, `go test` passes | `pocs/01-openshell-baseline/run.sh` |
| 03-credential-placeholder | F03: env holds placeholder, proxy injects real secret only at bound endpoint | `pocs/03-credential-placeholder/run.sh` |
| 04-opencode-in-sandbox | F04: OpenCode inside sandbox does edit → go test → commit | `pocs/04-opencode-in-sandbox/run.sh` |
| containment | F02: hostile-agent probes must be blocked | `tests/containment/run.sh` |

Prereqs: host setup from `ai-docs/sprints/01-openshell-eval/F01-openshell-baseline/review.md`.
