# F17 — z3r0 up / attach / down
Status: todo
Description: One command per repo: create-or-reuse sandbox, push repo in, run `opencode serve` inside, host TUI attaches. VS Code uses same sandbox.
Goal: `z3r0 up` in a repo → working OpenCode session whose tools all run in the sandbox.
Plan:
1. Spike: port-forward via `openshell sandbox ssh-config` + `ssh -L` to `opencode serve` inside → verify: `opencode attach http://127.0.0.1:<port>` works
2. scripts/z3r0 up|attach|down|status: name from repo, reuse if exists, git bundle in (no raw .git copy from host trust issue) → verify: idempotent re-run
3. VS Code: `z3r0 code` = `sandbox connect --editor vscode` on same sandbox → verify: edit visible in both
DONE: up/attach/down scripted; second `up` reuses; host files untouched.
GOOD: < 30 s to attach on reuse; serve bound to 127.0.0.1 inside only.
Proof: review.md runs up → prompt edits file → down; host-untouched check.
Log:
