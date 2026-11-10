# F12 — Push from sandbox
Status: done
Description: Code leaves only via git push to a branch from inside; host never downloads files (P01, P05).
Goal: Agent change appears as a branch/MR; host fetches via git, no fsmonitor/hooks execution.
Plan:
1. Remote reachable from sandbox (local bare repo via gateway? or GitLab) — decide → verify: push works, other hosts denied
2. Host fetch of pushed branch with malicious .git/config attempt → verify: fsmonitor not transferred (C20 closed)
DONE: push path scripted; C20/C21 path documented as closed for this flow.
GOOD: push restricted to `z3r0/*` branches.
Proof: attack reproduction from F08 step 4 against new flow → PASS.
Log:
- 2026-11-10: Step 1: remote = GitLab via forge token (D20); push from sandbox to branch + MR via push options works; main protected (push = No one). Step 2: in sandbox set core.fsmonitor + core.hooksPath, committed, pushed zero/f12-probe; host `git fetch` → host config unchanged, no hook ran (C20 closed for this flow). Fix on the way: sandbox git had no author identity → zero copies host user.name/email. GOOD (`zero/*` only) not enforced: any non-protected branch is pushable — accepted with D20 (token is user-accountable).
