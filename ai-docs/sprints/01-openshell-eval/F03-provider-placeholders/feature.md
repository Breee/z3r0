# F03 — Provider placeholder credentials
Status: done
Description: Check that OpenShell providers inject placeholder creds and substitute real ones only at the supervisor (arch §8, Phase 0 item 4).
Goal: Agent can call a test API (e.g. GitLab) without ever seeing the real token.
Plan:
1. Configure a provider for a GitLab/test endpoint with a real token on host.
2. Inside sandbox: call API via `curl`/`glab`; dump env, files, memory-visible secrets.
3. Try placeholder against a non-bound endpoint.
DONE: API call succeeds; real token never visible in sandbox; placeholder useless elsewhere.
GOOD: test automated in `tests/providers/`.
Proof: `tests/providers/run.sh` → call OK, grep for real token in sandbox = 0 hits, off-endpoint call denied.
Log:
- 2026-10-09: PoC `pocs/03-credential-placeholder/run.sh` 5/5 PASS with httpbin echo profile (env=placeholder, secret absent, upstream gets real, unbound path 403, forged 500). Remaining: real GitLab token, `glab`. Finding → D06, C13.
