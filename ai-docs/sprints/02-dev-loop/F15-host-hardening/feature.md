# F15 — Host hardening
Status: todo
Description: Shrink breakout blast radius: KEK, client key, docker group, disk quota (P03, P06).
Goal: blast-radius.sh as openshell finds nothing usable; disk fill capped.
Plan:
1. Remove bree from docker group or document accept → verify: blast-radius
2. Client key: restrict / short-lived → verify: C26 status
3. Disk quota for sandbox volumes → verify: 1G write fails at limit
DONE: blast-radius findings each fixed or accepted in decisions.md.
GOOD: script per fix in scripts/.
Proof: tests/breakout/blast-radius.sh bree|openshell, tests/attacks resources.
Log:
