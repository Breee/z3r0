# F14 — Short-lived GitLab creds
Status: todo
Description: Real GitLab token via provider placeholder; token minted short-lived and scoped (F03 rest, C26).
Goal: Agent can glab/git push with a token that expires in hours and is scoped to one project.
Plan:
1. Real token test: glab in sandbox via placeholder → verify: API ok, token not in sandbox
2. Mint/rotate script (project access token, expiry) → verify: expired token rejected
DONE: glab works with placeholder; token TTL ≤ 1 day.
GOOD: rotation automatic on `z3r0 up`.
Proof: review.md commands, `glab api user` inside sandbox.
Log:
