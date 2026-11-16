# Pause and resume of a sandbox (#104)

Tested 2026-11-16, OpenShell 0.1.8, dev image, throwaway sandboxes; zero-level check on `zero-developer`.

| Check | podman driver | VM driver |
|---|---|---|
| `sandbox stop` / `start` | works, ~6 s + ~1 s | works, ~5 s + a few s |
| Files in `$HOME` after stop/start | kept | kept |
| Files in `/tmp` after stop/start | kept | **lost** |
| Running processes after stop/start | gone (restart needed) | gone |
| ssh with the old ssh config after start | works | works (config re-read in test) |
| `sandbox suspend` / `resume` | refused: `compute driver does not support sandbox suspend` | refused, same message |
| Credential rotated while stopped (`provider update`) | first new session has the new placeholder | not tested |

zero level (podman): after `sandbox stop` + `sandbox start`, `zero up` is ready in 2 s on the same port; checkout, branch, opencode database, git, glab, push gate all work.

## Findings

- Stop/start is the usable pause. No process survives, so the stale-token problem of rotation (C44) does not exist after a start.
- Suspend/resume (memory kept) is not available on either driver in 0.1.8. The CLI prints the error but exits 0.
- VM driver loses `/tmp` on stop. zero keeps the checkout in `/tmp/zero/<repo>` → on the VM driver a pause would lose all unpushed work (C50).
- `zero status` prints `sandbox …: up` for a stopped sandbox (only checks that it exists).

## Not tested

`zero up` on a stopped sandbox without a manual `sandbox start`; token revoke on stop and mint on start through zero; reopening an opencode session in the UI after start; credential rotation on the VM driver.

## Recommendation

Build `zero pause` (close tunnel, stop sandbox, revoke token) and let `zero up` start a stopped sandbox and mint a fresh token. The token then lives only while the session is active. Before that works on the VM driver the checkout must move out of `/tmp`.
