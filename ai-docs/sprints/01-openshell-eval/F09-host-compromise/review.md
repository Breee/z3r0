# F09 — Review
## Step 1: blast radius as current user (simulated breakout)
```
tests/breakout/blast-radius.sh
```
Expected: `FOUND` lines (names only, no secrets printed); `# total findings: 14` on this laptop, incl. gateway-db, gateway-kek, ssh-key, podman-socket, docker-socket, group docker.
## Step 2: gateway + podman as dedicated user `openshell`
```
sudo useradd -m -s /usr/sbin/nologin openshell
sudo loginctl enable-linger openshell
grep openshell /etc/subuid /etc/subgid
sudo scripts/gateway-dedicated-user.sh bree      # revert: ... bree --revert
openshell status
ps -o user=,cmd= -C openshell-gateway
pocs/01-openshell-baseline/run.sh
pocs/03-credential-placeholder/run.sh
tests/containment/run.sh f09                      # host check uses scripts/podman-gw.sh (sudo)
```
Expected: subuid/subgid lines; status Connected (mTLS); gateway process user `openshell`; PoC01 PASS; PoC03 5 PASS + 2 known FAIL (C18); containment 46 PASS.
## Step 3: blast radius after move
```
rm -rf ~/.local/state/openshell/gateway && systemctl --user disable --now podman.socket
tests/breakout/blast-radius.sh openshell
tests/breakout/blast-radius.sh
```
Expected: as openshell only gateway-db, gateway-kek (+ its podman socket); as bree no gateway-db/kek, still openshell-client-key (C26), ssh/tokens, docker group (C25).
