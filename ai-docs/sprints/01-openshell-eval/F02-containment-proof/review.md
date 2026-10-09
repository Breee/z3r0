# F02 — Review

## Step 1+2: harness + filesystem/child-process cases
```sh
tests/containment/run.sh            # creates sandbox f02, runs probe, deletes it
echo "exit=$?"
openshell sandbox list | grep f02   # nothing: cleaned up
ls ~/.z3r0-canary                   # No such file: canary removed
```
Expected: `PASS control:read-workspace`, then 13 `PASS fs:*`/`proc:*` lines, no `FAIL`, `exit=0`.
Note: host canary `~/.z3r0-canary` is grepped for across the sandbox root fs (must not be found).

## Step 3: privilege cases
```sh
tests/containment/run.sh | grep -E 'priv|FAIL'
# manual look at the evidence:
scripts/sandbox-up.sh chk tests/fixtures/hello-go
openshell sandbox exec -n chk -- sh -c 'id; grep -E "Cap|NoNew|Seccomp:" /proc/self/status; ls -l /bin/su'
scripts/sandbox-down.sh chk
```
Expected: 11 `PASS priv:*`, no `FAIL`. Manual: uid=1000, all Cap* = 0, `NoNewPrivs: 1`, `Seccomp: 2`, `/bin/su` is setuid (C12) but unusable.

## Step 4: host container engine + mounts
```sh
tests/containment/run.sh | grep -E 'podman|mount:|host:|FAIL'
```
Expected: 10 `PASS podman:*`, 2 `PASS mount:*`, `PASS host:no-extra-bind-mounts`, no `FAIL`. Whole harness: 38 PASS, exit 0.

## Step 5 — egress (default policy = deny-all)
```bash
openshell sandbox delete f02; sleep 8
tests/containment/run.sh | grep -E 'net:|FAIL'
openshell sandbox exec -n f02 -- sh -c 'cat /etc/resolv.conf; getent hosts x.z3r0test.invalid'
bash pocs/03-credential-placeholder/run.sh   # allowlist case: httpbin reachable only via policy
```
Expected: all `net:*` PASS, total 46/46, exit 0. DNS answers are synthetic 198.18.x.x (even for .invalid) → no upstream DNS query, no DNS exfil.
