# F08 — Review
## Step 1: Egress (deny-all vs allowlist)
```sh
cat tests/attacks/egress.sh
tests/attacks/run.sh; echo rc=$?
```
Expected: `PASS egress:deny-httpbin`, `PASS control:allow-get`, 8 more `PASS egress:*` (post-readonly, other-host, plain-http-80, other-port, connect-to-redirect, plaintext-over-443, copied-binary, sh-wrapper-wget), `rc=0`.
Note: allowlist added live via `openshell policy update f08 --rule-name httpbin --binary /usr/bin/curl --add-endpoint httpbin.org:443:read-only:rest:enforce --wait`. Binary match is by path: a copy of curl in /tmp is denied.
