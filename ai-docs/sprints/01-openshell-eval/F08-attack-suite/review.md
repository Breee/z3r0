# F08 — Review
## Step 1: Egress (deny-all vs allowlist)
```sh
cat tests/attacks/egress.sh
tests/attacks/run.sh; echo rc=$?
```
Expected: `PASS egress:deny-httpbin`, `PASS control:allow-get`, 8 more `PASS egress:*` (post-readonly, other-host, plain-http-80, other-port, connect-to-redirect, plaintext-over-443, copied-binary, sh-wrapper-wget), `rc=0`.
Note: allowlist added live via `openshell policy update f08 --rule-name httpbin --binary /usr/bin/curl --add-endpoint httpbin.org:443:read-only:rest:enforce --wait`. Binary match is by path: a copy of curl in /tmp is denied.
## Step 2: Exfil via allowed endpoint + placeholder laundering
```sh
cat tests/attacks/exfil.sh
tests/attacks/run.sh; echo rc=$?
grep -n "C17\|C19" ai-docs/caveats.md
```
Expected: step 1 lines as before, then `PASS launder:{other-host-header,other-host-query,body,path}`, `GAP launder:{query,other-header,basic-auth} (C17)`, `GAP exfil:data-in-query (C19)`, `rc=0`.
## Step 3: Gateway / supervisor
```sh
cat tests/attacks/gateway.sh
tests/attacks/run.sh; echo rc=$?
```
Expected: `PASS control:pid1-is-supervisor`, 16 `PASS sup:*`/`gw:*`, no FAIL, `rc=0`.
## Step 4: Malicious workspace → host via download
```sh
cat tests/attacks/download.sh
tests/attacks/run.sh; echo rc=$?
grep -n "C20\|C21" ai-docs/caveats.md
```
Expected: `GAP dl:abs-symlink-kept (C21)`, `GAP dl:git-fsmonitor-host-exec (C20)`, `PASS dl:write-through-symlink`, `PASS dl:relative-traversal`, `rc=0`.
