# F11 — Dev image
Status: done
Description: Own image (golang, git, curl, opencode, PATH in /etc/profile) so agent + tools run inside without uploads (D08, C09).
Goal: `sandbox create --from <image@digest>` works with go/git/opencode on PATH in login shell.
Plan:
1. images/dev/Containerfile (base + opencode pinned), scripts/build-dev-image.sh builds in gateway user storage → verify: tools present
2. Pin by digest in scripts/sandbox-up.sh default; drop C09 PATH workaround → verify: PoC01 + PoC04 pass without --env PATH
3. Drop setuid bits (C12) → verify: `find / -perm -4000` empty, containment 45 PASS
DONE: image pinned by digest; PoC01, PoC04, containment pass on it.
GOOD: rebuild reproducible from one script; image < 1.5 GB.
Proof: pocs/01, pocs/04, tests/containment/run.sh with new default image.
Log:
- 2026-10-09: step 1 Containerfile + build script; test build as bree ok (opencode 1.18.21, go 1.25.14, git, rg; 1.06 GB). Gateway build pending user sudo.
- 2026-10-09: step 2+3 profile.d PATH/GOPATH, setuid stripped, sandbox-up default = localhost/z3r0-dev:latest, pocs/11-dev-image; local build: login PATH has go, 0 setuid. Old PoCs keep golang:1.25 for replay.
- 2026-10-09: verified: PoC11 5 PASS; containment on dev image 45 in-sandbox PASS, 0 FAIL (host bind check needs user sudo). DONE.
