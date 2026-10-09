# Caveats

Format: `CNN — <area> — <issue>` + impact/mitigation.

- C01 — OpenShell — All capabilities in arch doc Appendix A are assumptions, not verified. Must be proven in Sprint 01.
- C02 — Platform — Landlock/seccomp are Linux-only; macOS/Windows gaps unknown.
- C03 — OpenShell — Workspace is uploaded (copy), not bind-mounted; edits need sync back to host repo or git push from sandbox.
- C04 — OpenShell — Telemetry on by default; set `OPENSHELL_TELEMETRY_ENABLED=false`.
- C05 — Host — Podman 6.1.3 is manual install in /usr/local/bin: outside OpenShell's documented 5.x, no package signature/updates. Verify origin, update manually.
- C06 — Host — Podman user socket = full user authority. Agent must never reach it (proved in F02).
- C07 — Host — AppArmor userns exception for /usr/local/bin/podman (mirrors Ubuntu's podman profile).
- C08 — Host — Image policy allowlists registries but does not verify signatures. Add sigstore/GPG verification later.
- C09 — OpenShell — `sandbox exec` uses login shell; image `ENV PATH` is lost (golang image: pass `--env PATH=/usr/local/go/bin:...`). Fix via custom image/profile.
- C10 — OpenShell — `sandbox upload <dir> <dest>` creates `<dest>/<basename>`; respects .gitignore. Copy, not sync (confirms C03).
- C11 — OpenShell — `/proc/mounts` in sandbox leaks host paths (`/home/<user>/.local/share/containers/...`) → username leak. Low risk; no content access.
- C12 — Image — setuid binaries (`su`, `passwd`) present in golang image; neutralised by NoNewPrivs=1 + empty CapBnd + Seccomp=2. Prefer minimal images.
- C13 — OpenShell — Gateway runs as the login user; provider secrets in `~/.local/state/openshell/gateway/openshell.db` are encrypted, but `credentials/key-encryption-key.bin` is next to it, same owner. Breakout → all secrets. Fix: F09.
- C14 — OpenShell — `sandbox delete` is async; re-creating the same name immediately can fail (harness printed 0 results once). Wait or use unique names.
- C15 — OpenShell — Credential rewrite only for HTTP the proxy inspects; `tls: skip`/non-HTTP endpoints get no rewrite (and no L7 check).
- C16: sandbox DNS is synthetic (198.18.0.0/15); tools that validate resolved IPs or need real DNS (SRV, TXT) may break.
- C17 (medium): placeholder laundering. Proxy resolves the placeholder in query, ANY header (not only the bound one) and basic auth on the bound endpoint (F08: launder:*); body and path not. If the bound API echoes or stores that input (echo service, issue title via query, custom header logged), the real secret comes back to the agent. Proxy does not redact secrets in responses. Do not bind creds to debug/echo endpoints.
- C18 (high): foreign-key exfil. Agent sends its own/attacker key (Authorization, X-Api-Key, ?api_key) to an allowed endpoint → data lands in attacker account. Proven by PoC03 foreign-* FAIL. OpenShell rules match method/path/query only, no header matchers. Mitigation: query allowlist; headers need upstream feature or our own egress/LLM gateway that strips+injects auth.
- C19 (inherent): every allowed endpoint is an exfil channel (data in query/path/body). Only mitigation: few endpoints, read-only, tight path/query rules, Z3R0 audit.
- C20 (high): `sandbox download` of a git repo copies `.git/config` + hooks. `git status` on host then runs agent code (core.fsmonitor proven, F08 dl:git-fsmonitor-host-exec). Rule: never download repos to host; agent output reaches host only via git push → GitLab MR review (fetch, don't copy .git).
- C21 (medium): `sandbox download` keeps absolute symlinks to host paths (e.g. ~/.ssh). Later tools on host may follow them. Write-through on re-download and relative traversal are blocked.
