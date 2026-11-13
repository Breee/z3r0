# F16 — MicroVM driver trial
Status: done (2026-11-13) — VM driver works on OpenShell 0.1.8; podman stays default, `zero up --vm` opt-in; container build still blocked (C48)
Description: Run sandboxes under `compute_driver = "vm"` (libkrun, ships in 0.1.2) for kernel isolation (arch E3; P03, P09, C13, C24).
Goal: Same PoCs pass under the VM driver; startup/memory cost known; guest has no NIC (all egress via supervisor).
Plan:
1. Host prep (D15, keep podman gateway): `usermod -aG kvm openshell`, second gateway config with `compute_driver = "vm"` (+ `[openshell.drivers.vm]` vcpus/mem_mib/overlay_disk_mib) on its own port, start as second service, `openshell gateway add openshell-vm` → verify: `openshell -g openshell-vm sandbox create` Ready with `localhost/z3r0-dev:latest` (rootfs tar staging); podman gateway still serves.
2. `tests/containment/run.sh` under VM → verify: same 45/0 or diff logged; `ip link` in guest shows no NIC besides lo/vsock.
3. PoC01, PoC03 (placeholders), PoC04 (zero up) under VM → verify: pass table diff vs podman.
4. Measure: `time openshell sandbox create`, RSS of krun process, disk cap via `overlay_disk_mib` (write until ENOSPC, host untouched) → verify: numbers in notes.md.
5. zero: `--vm` flag / `Z3R0_DRIVER` → `-g openshell-vm`; default stays podman (D15) → verify: `zero up --vm` ready, `zero status` shows driver.
DONE: comparison table in notes.md; gateway.toml snippet in review.md; feature matrix row `MicroVM driver` → tested.
GOOD: startup < 5s, mem overhead < 512 MiB idle; C22 disk quota closed.
Proof: steps 2–3 scripts unchanged, VM driver active, pass table ≥ podman.
Log:
- 2026-11-10: Re-tested on 0.1.4 (vm06 alpine): same #4014 failure. Comment drafted: upstream/openshell-vm-proc-status.md. Steps 2–5 blocked; F18 step 3 blocked with it.
