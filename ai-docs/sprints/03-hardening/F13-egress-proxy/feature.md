# F13 — Egress / LLM proxy
Status: todo
Description: Proxy outside sandbox (non-bree user) strips client auth, injects ours; closes foreign-key + laundering (P02, P04, D07).
Goal: Attacker key and query placeholders never reach upstream.
Plan:
1. Option A: per-endpoint deny rules on auth query params → verify: C27 attack blocked
2. Option B: own proxy (e.g. LiteLLM) for LLM traffic → verify: PoC03 foreign-key checks PASS
3. Decide A/B/both → decisions.md
DONE: PoC03 7/7 PASS; F04 attack.sh real-secret-in-agent-context PASS.
GOOD: proxy runs as dedicated user, config in repo.
Proof: pocs/03/run.sh, pocs/04-opencode-in-sandbox/attack.sh.
Log:
