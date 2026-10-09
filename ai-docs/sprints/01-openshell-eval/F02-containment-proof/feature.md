# F02 — Containment proof (fs, process, egress)
Status: todo
Description: Verify OpenShell contains a corrupted agent (arch §25 Phase 0 items 1–3).
Goal: Automated test harness acting as a hostile agent inside the sandbox.
Plan:
1. Write `tests/containment/` scripts run inside sandbox.
2. Cases: read `~/.ssh`, `~/.kube`, `~/.config/gh`, other repos; spawn child (`sh -c`, `python`, background) and repeat; curl unknown host; DNS exfil.
3. Run once with deny-all egress, once with allowlist.
DONE: all hostile cases fail; allowlisted destination succeeds.
GOOD: harness is one command, outputs pass/fail table; gaps logged in caveats.md.
Proof: `tests/containment/run.sh` → all PASS (= blocked).
Log:
