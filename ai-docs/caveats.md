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
