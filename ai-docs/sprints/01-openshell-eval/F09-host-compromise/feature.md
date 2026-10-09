# F09 — Host compromise blast radius + hardening
Status: done
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
- 2026-10-09: step 1 `tests/breakout/blast-radius.sh` as bree → 14 findings: gateway db+KEK, openshell client key, ssh key+agent, gh/glab/docker tokens, gnupg, podman socket, docker socket + docker group (= host root), 5 repos, browser cookies. Breakout as bree = root (C25).
- 2026-10-09: step 2 `scripts/gateway-dedicated-user.sh` → gateway runs as openshell, CLI via mTLS client cert; PoC01 PASS, PoC03 5/5 (+2 known C18 FAIL), containment 45 in-sandbox PASS. Test host-side podman checks now via `scripts/podman-gw.sh` (sudo).
- 2026-10-09: step 3 as bree after cleanup → 12 findings, gateway db/KEK gone; left: mTLS client key (C26), stale podman.sock file (service inactive), ssh/tokens/docker group (C25). Run as openshell: user ran (results not pasted).
- 2026-10-09: step 4 (docs only): MicroVM driver ships in 0.1.2 (`/usr/libexec/openshell/openshell-driver-vm`, libkrun, `compute_driver = "vm"`, opt-in). Own guest kernel (closes C24, mitigates kernel CVE C13), no NIC in VM (all traffic via supervisor), `overlay_disk_mib` caps disk (C22), `vcpus`/`mem_mib`. Needs /dev/kvm: openshell user must join group kvm (/dev/kvm root:kvm 660). Not trialled → follow-up sprint. Patch policy: kernel via Ubuntu updates; Podman 6.1.3 manual (C05) → watch upstream releases.
- 2026-10-09: step 5: no refresh demo; short-lived creds (GitLab token expiry, OAuth/STS providers) moved to sprint 2 with F03 GitLab test. F09 done.
