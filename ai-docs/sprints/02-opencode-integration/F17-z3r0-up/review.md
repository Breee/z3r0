# F17 — Review

## Step 1: spike — host opencode attaches to `opencode serve` inside sandbox via ssh -L
```sh
N=s17
scripts/sandbox-up.sh $N tests/fixtures/hello-go
openshell policy update $N --rule-name zen --binary /usr/local/bin/opencode --add-endpoint opencode.ai:443:read-write:rest:enforce --wait
openshell sandbox ssh-config $N > /tmp/s17.conf
ssh -F /tmp/s17.conf -fN -L 14096:127.0.0.1:4096 openshell-$N.default
ssh -F /tmp/s17.conf openshell-$N.default 'cd /sandbox/hello-go && nohup opencode serve --hostname 127.0.0.1 --port 4096 >/tmp/serve.log 2>&1 &'
sleep 8
opencode run --attach http://127.0.0.1:14096 -m opencode/nemotron-3.5-lightning-free 'Use the bash tool to run: id -u && pwd && hostname. Then use the write tool to create attached.txt containing hi.'
ls attached.txt; openshell sandbox exec -n $N -- cat /sandbox/hello-go/attached.txt
opencode attach http://127.0.0.1:14096     # optional: interactive TUI
openshell sandbox delete $N                # also ends the ssh forward
```
Expected: bash output `1000`, `/sandbox/hello-go`, `sandbox-s17`; host `ls` fails; sandbox cat prints `hi`.

## Step 2
```sh
Z=$PWD/scripts/z3r0
R=/tmp/z3r0-demo; rm -rf $R; cp -r tests/fixtures/hello-go $R; cd $R
git init -q && git add . && git -c user.email=a@b -c user.name=t commit -qm init
$Z up; $Z up                      # 2nd run reuses
P=$(cat ~/.local/state/z3r0/z3r0-z3r0-demo/port)
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:$P/config   # no password
$Z run -m opencode/nemotron-3.5-lightning-free "Run id -u; hostname; pwd; go test ./... Then create attached.txt with content ok."
ls attached.txt; openshell sandbox exec -n z3r0-z3r0-demo -- cat /sandbox/z3r0-demo/attached.txt
$Z stop; $Z status; $Z attach     # TUI, sandbox kept; quit with ctrl+c
$Z rm
```
Expected: both `up` print `ready`; curl `401`; agent reports `1000`, `sandbox-z3r0-demo`, `/sandbox/z3r0-demo`, tests ok; host `ls` fails; sandbox cat `ok`; status after stop `forward: off`; attach opens TUI on same session.
