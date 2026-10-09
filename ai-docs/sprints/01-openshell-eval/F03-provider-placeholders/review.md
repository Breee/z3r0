# F03 — Review

## Step 1: placeholder PoC (echo endpoint)
```sh
cat pocs/03-credential-placeholder/profile.yaml   # credential ECHO_TOKEN bound to httpbin.org/anything, curl only
pocs/03-credential-placeholder/run.sh; echo "exit=$?"
openshell profile list; openshell provider list   # cleaned up
```
Expected: 5 PASS (env-is-placeholder, secret-not-in-sandbox, upstream-gets-real-secret, unbound-path-denied, forged-placeholder-denied), exit=0, no leftovers. Needs internet.

## Breakout finding (manual, host side)
```sh
ps -o user= -p "$(systemctl --user show openshell-gateway -p MainPID --value)"
ls -l ~/.local/state/openshell/gateway/ ~/.local/state/openshell/gateway/credentials/
```
Expected: gateway runs as you; `key-encryption-key.bin` owned by you next to `openshell.db` → C13/D06.
