# F05 — Review

## Step 1: Remote-SSH plumbing (scripted)
```sh
pocs/05-vscode-remote/run.sh vsc05
```
Expected: `PASS ssh-non-root`, `PASS curl-to-copilot-denied` (curl is not a VS Code server binary), `PASS server-download-denied`, rc=0. Sandbox stays up.

## Step 2: Copilot Chat inside the sandbox (manual GUI)
```sh
code --remote ssh-remote+openshell-vsc05.default /sandbox/hello-go
```
VS Code settings (user): `"remote.SSH.configFile": "~/.ssh/openshell-vsc05.conf"`, `"remote.SSH.localServerDownload": "always"` (server is fetched on host, sandbox has no egress to update.code.visualstudio.com).
Then in Copilot Chat (Agent mode):
1. "Add a Goodbye() func with test, run go test, git commit." → commit inside sandbox:
```sh
openshell sandbox exec -n vsc05 -- git -C /sandbox/hello-go log --oneline -1
```
2. "Run `sh /sandbox/containment/probe.sh /home/$USER ~/.z3r0-canary 1000` and show the summary." Expected: 0 FAIL.
3. Denials seen by the proxy (missing Copilot hosts show here):
```sh
openshell logs vsc05 2>&1 | grep -i deny | tail
```
Cleanup: `scripts/sandbox-down.sh vsc05; rm ~/.ssh/openshell-vsc05.conf`

## Result of step 2 (2026-10-09)
```sh
openshell sandbox exec -n vsc05 --env PATH=/usr/local/go/bin:/usr/bin:/bin -- sh -c 'cd /sandbox/hello-go && git log --oneline -2 && go test ./...'
openshell logs vsc05 2>&1 | grep DENIED | grep -oE "\-> [a-z0-9.-]+" | sort | uniq -c
```
Expected: Copilot commit `Add Goodbye function` on top of `init`; `ok example.com/hello`; denied only telemetry (mobile.events.data.microsoft.com, telemetry.enterprise.githubcopilot.com) + probe targets. Copilot reported probe 45 PASS 0 FAIL.
