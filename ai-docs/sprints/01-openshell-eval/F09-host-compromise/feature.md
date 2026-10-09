# F09 — Host compromise blast radius + hardening
Status: todo
Description: Assume a container breakout (arch §14.14 "compromised workstation"). Measure what the agent gets on the host and harden (D06, C13).
Goal: Breakout lands in a dedicated low-value host user; real secrets not reachable from it.
Plan:
1. Simulate breakout: run probe as the gateway's host user (no real exploit) → list reachable secrets (KEK+db, ~/.ssh, tokens, podman socket, other repos).
2. Move gateway + rootless podman to dedicated user `openshell` (systemd user service, linger); CLI talks via mTLS only.
3. Re-run step 1 as `openshell` → expect nothing of value; check what the CLI user still exposes (client certs).
4. Evaluate VM/microVM driver option in OpenShell 0.1.2 (docs only) + kernel/podman patch policy (C05).
5. Keep laptop creds short-lived: provider refresh (OAuth/STS) demo or note for later sprint.
DONE: step-1 list before/after documented; gateway runs as `openshell`; F01–F03 PoCs still pass.
GOOD: setup scripted in `scripts/`, reversible; residual risk in caveats.md.
Proof: `tests/breakout/blast-radius.sh <user>` → before: secrets listed; after: none.
Log:
