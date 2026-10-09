# F12 — Push from sandbox
Status: todo
Description: Code leaves only via git push to a branch from inside; host never downloads files (P01, P05).
Goal: Agent change appears as a branch/MR; host fetches via git, no fsmonitor/hooks execution.
Plan:
1. Remote reachable from sandbox (local bare repo via gateway? or GitLab) — decide → verify: push works, other hosts denied
2. Host fetch of pushed branch with malicious .git/config attempt → verify: fsmonitor not transferred (C20 closed)
DONE: push path scripted; C20/C21 path documented as closed for this flow.
GOOD: push restricted to `z3r0/*` branches.
Proof: attack reproduction from F08 step 4 against new flow → PASS.
Log:
