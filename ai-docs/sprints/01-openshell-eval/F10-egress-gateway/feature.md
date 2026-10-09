# F10 — Own egress/LLM gateway (foreign-key mitigation)
Status: todo
Description: Sandbox may only reach our gateway for LLM/API traffic; gateway strips client auth and injects ours. Closes C18 until OpenShell supports it (ai-docs/upstream/openshell-foreign-credential-passthrough.md).
Goal: Attacker key sent from sandbox never reaches upstream.
Plan:
1. Pick gateway (LiteLLM or minimal reverse proxy); run on host as non-bree user → verify: health check.
2. OpenShell policy: only gateway host allowed; query allowlist on remaining endpoints.
3. Re-run PoC03 foreign-* cases against gateway path → verify: PASS.
DONE: foreign-* checks PASS; legit agent call works.
GOOD: gateway holds the only real key; logs per-request principal; no extra open ports.
Proof: `sh pocs/03-credential-placeholder/run.sh` (foreign-* PASS) + gateway log shows stripped header.
Log:
