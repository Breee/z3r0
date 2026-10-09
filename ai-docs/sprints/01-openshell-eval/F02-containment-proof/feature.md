# F02 — Containment proof (fs, process, egress)
Status: doing
Description: Verify OpenShell contains a corrupted agent (arch §25 Phase 0 items 1–3).
Goal: Automated test harness acting as a hostile agent inside the sandbox.
Plan:
1. Write `tests/containment/` scripts run inside sandbox.
2. Cases: read `~/.ssh`, `~/.kube`, `~/.config/gh`, other repos; spawn child (`sh -c`, `python`, background) and repeat; curl unknown host; DNS exfil.
3. Privilege cases: agent uid≠0, no `sudo`/setuid escalation, no caps (`capsh --print`), `no_new_privs=1`, cannot write `/etc`.
4. Podman cases: no podman/docker socket reachable (`$XDG_RUNTIME_DIR/podman/podman.sock`, `/run/podman`, `/var/run/docker.sock`), `podman`/`docker` CLI from sandbox cannot talk to host engine, cannot mount host paths.
5. Run once with deny-all egress, once with allowlist.
DONE: all hostile cases fail (incl. root + host podman access); allowlisted destination succeeds.
GOOD: harness is one command, outputs pass/fail table; gaps logged in caveats.md.
Proof: `tests/containment/run.sh` → all PASS (= blocked).
Log:
- 2026-10-09: step 1+2 — harness (run.sh host, probe.sh in sandbox, canary + control); 13/13 fs/proc cases blocked. Egress/DNS cases moved to step 5.
