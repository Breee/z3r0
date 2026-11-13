---
status: accepted
date: 2026-11-13
deciders: bree
requirements: [Z-INV-1]
---
# 0029 — GitHub Copilot as model provider inside the sandbox

## Context
The agent in the sandbox needs a model. The host's opencode is logged in to GitHub Copilot; the free Zen tier is rate-limited. The sandbox must never hold a real secret.

## Decision
- On `zero up` (create), if the host's `opencode auth export` has an active `github-copilot` OAuth credential, zero binds its token as provider `<sandbox>-copilot` (profile `zero-copilot`): bearer on `api.githubcopilot.com:443`, opencode binary only.
- The sandbox gets the placeholder in `COPILOT_TOKEN`; zero imports it as opencode's Copilot credential (`opencode auth import`). The proxy substitutes the real token.
- No Copilot login on the host: nothing is bound, sandbox works as before.
- `zero rm` deletes the provider.

## Consequences
- The token is the user's long-lived Copilot OAuth token (`gho_`), stored in the OpenShell gateway like forge tokens. It is not minted per sandbox and not revoked on `rm`; GitHub offers no narrower Copilot token.
- The agent can spend the user's Copilot quota and send repo content to Copilot models (same exfil class as any model API, P-paths in `ai-docs/attack-paths.md`).
- Host token change (re-login) is picked up only on sandbox recreate.
- Provider profile `binaries` must be the real executable path, not the `/usr/local/bin/opencode` symlink (OpenShell 0.1.8); `zero-gitlab` fixed the same way.
