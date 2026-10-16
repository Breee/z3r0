# F16 — MicroVM driver trial
Status: todo
Description: Run sandboxes under `compute_driver = "vm"` (libkrun, ships in 0.1.2) for kernel isolation (arch E3; P03, P09, C13, C24).
Goal: Same PoCs pass under the VM driver; startup/memory cost known; guest has no NIC (all egress via supervisor).
Plan:
1. Host prep: `usermod -aG kvm openshell`, gateway.toml `compute_driver = "vm"` (+ `[openshell.drivers.vm]` vcpus/mem_mib/overlay_disk_mib), restart gateway → verify: `openshell sandbox create` Ready with `localhost/z3r0-dev:latest` (rootfs tar staging).
2. `tests/containment/run.sh` under VM → verify: same 45/0 or diff logged; `ip link` in guest shows no NIC besides lo/vsock.
3. PoC01, PoC03 (placeholders), PoC04 (zero up) under VM → verify: pass table diff vs podman.
4. Measure: `time openshell sandbox create`, RSS of krun process, disk cap via `overlay_disk_mib` (write until ENOSPC, host untouched) → verify: numbers in notes.md.
5. Decide driver default for zero (D13) and whether both drivers stay (`Z3R0_DRIVER`).
DONE: comparison table in notes.md; gateway.toml snippet in review.md; feature matrix row `MicroVM driver` → tested.
GOOD: startup < 5s, mem overhead < 512 MiB idle; C22 disk quota closed.
Proof: steps 2–3 scripts unchanged, VM driver active, pass table ≥ podman.
Log:
