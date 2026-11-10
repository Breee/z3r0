# F14 — Short-lived GitLab creds
Status: done
Description: Real GitLab token via provider placeholder; token minted short-lived and scoped (F03 rest, C26).
Goal: Agent can glab/git push with a token that expires in hours and is scoped to one project.
Plan:
1. Real token test: glab in sandbox via placeholder → verify: API ok, token not in sandbox
2. Mint/rotate script (project access token, expiry) → verify: expired token rejected
DONE: glab works with placeholder; token TTL ≤ 1 day.
GOOD: rotation automatic on `zero up`.
Proof: review.md commands, `glab api user` inside sandbox.
Log:
- 2026-11-10: Done via D20/forge: `zero up` mints a fine-grained project token (expires next day, scoped to one project + execute_mcp_tool), stored only as OpenShell provider; sandbox sees placeholder; `zero rm` revokes (verified 401 after). glab works in sandbox (`glab mr list` after D22). `glab api user` is 403 by design (no user scope). Rotation = recreate (C35).
