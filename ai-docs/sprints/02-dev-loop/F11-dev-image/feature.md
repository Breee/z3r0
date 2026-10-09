# F11 — Dev image
Status: todo
Description: Own image (golang, git, curl, opencode, PATH in /etc/profile) so agent + tools run inside without uploads (D08, C09).
Goal: `sandbox create --from <image@digest>` works with go/git/opencode on PATH in login shell.
Plan:
1. Containerfile in images/dev/, build rootless, push to allowed registry (or local) → verify: policy.json accepts only it
2. Pin by digest in scripts/sandbox-up.sh default; drop C09 PATH workaround → verify: PoC01 + PoC04 pass without --env PATH
3. Drop setuid bits (C12) → verify: `find / -perm -4000` empty, containment 45 PASS
DONE: image pinned by digest; PoC01, PoC04, containment pass on it.
GOOD: rebuild reproducible from one script; image < 1.5 GB.
Proof: pocs/01, pocs/04, tests/containment/run.sh with new default image.
Log:
