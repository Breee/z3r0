# F16 — MicroVM driver trial
Status: todo
Description: Try compute_driver="vm" (libkrun) for kernel isolation (P03, P09).
Goal: Same PoCs pass under VM driver; know cost.
Plan:
1. openshell in group kvm, enable vm driver → verify: sandbox create works
2. Run PoC01, containment, attacks → verify: results diff vs podman
DONE: comparison table in notes.md.
GOOD: startup time + memory measured.
Proof: same scripts, VM driver.
Log:
