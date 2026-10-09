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
