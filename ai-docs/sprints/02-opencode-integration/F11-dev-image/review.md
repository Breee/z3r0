# F11 — Review

## Step 1: build dev image into gateway user's storage
```sh
cat images/dev/Containerfile
scripts/build-dev-image.sh            # sudo: builds as user openshell
scripts/podman-gw.sh run --rm localhost/z3r0-dev:latest sh -c 'opencode --version; git --version; go version; rg --version | head -1'
```
Expected: image ID + size ~1.06 GB; `1.18.21`, `git version 2.47.3`, `go1.25.14`, `ripgrep 14.1.1`. Build fails if opencode sha256 mismatches.

## Step 2+3: PATH fix (C09), no setuid (C12), default image — rebuild first
```sh
scripts/build-dev-image.sh
pocs/11-dev-image/run.sh
tests/containment/run.sh f11 localhost/z3r0-dev:latest   # sudo for host check
```
Expected: 5 PASS (go-test-no-path-env, opencode-on-path, git-rg, no-setuid, non-root), rc 0; containment all PASS, 0 FAIL.
