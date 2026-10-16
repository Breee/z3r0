# F18 — Review

## Step 1: E1 baseline — classic builders fail
```
openshell sandbox exec zero-z3r0 -- sh -c 'command -v buildah podman docker nerdctl; echo ---; unshare --user --map-root-user true; echo unshare_exit=$?; grep -E "CapEff|NoNewPrivs|Seccomp" /proc/self/status'
```
Expected: all builders `absent`; `unshare: Operation not permitted`, exit 1; `CapEff: 0000000000000000`, `NoNewPrivs: 1`, `Seccomp: 2`.

## Step 2: ko build in E1 (namespace-free, daemon-free)
```
openshell sandbox create --name f18-test --from localhost/z3r0-dev:latest --policy pocs/06-sandbox-build/policy-registry.yaml --detach
openshell sandbox upload f18-test /home/bree/go/bin/ko /tmp/ko --no-git-ignore
openshell sandbox exec f18-test -- sh -c 'mkdir -p /tmp/komain && cd /tmp/komain && printf "module example.com/komain\n\ngo 1.25\n" > go.mod && printf "package main\n\nfunc main() { println(\"hi\") }\n" > main.go && chmod +x /tmp/ko && KO_DOCKER_REPO=registry.invalid/unused PATH=/usr/local/go/bin:$PATH HOME=/go /tmp/ko build --bare --sbom=none --tarball /tmp/komain.tar ./...'
```
Expected: `Saved /tmp/komain.tar`; then publish error `dial tcp … connect: permission denied` on registry.invalid (exit 1 — image cannot leave the sandbox; artifact already on disk). Host check:
```
podman images | grep -iE 'komain|unused' || echo "no such image on host"
```
Expected: `no such image on host`.
