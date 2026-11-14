# PoC 11: push service limits the agent to its session branch (#93)

`cmd/zero-push` is a git smart-HTTP proxy on the host. The sandbox may fetch from gitlab.com directly, but `git-receive-pack` to gitlab.com is denied by the OpenShell profile. Pushes go to zero-push, which reads the ref updates and forwards only the session branch. It holds no credential: the supervisor substitutes the sandbox token as usual and zero-push passes it on.

Run: `BIND=127.0.0.1 pocs/11-push-service/run.sh` (OpenShell 0.1.8, podman gateway, gitlab.com).

| Check | Result |
|---|---|
| clone/fetch directly from gitlab.com | PASS |
| glab API in sandbox | PASS |
| direct push to gitlab.com (403 from OpenShell) | PASS |
| other branch, main, mixed push denied, nothing created | PASS |
| session branch pushed, `-o merge_request.create` works, update works | PASS |
| delete of session branch denied | PASS |
| agent sees only placeholder | PASS |
| decision log (ref, old, new, decision, reason) | PASS |

Service bound to 127.0.0.1 works (supervisor runs in host network).

## Limits

- Force-push to the own session branch is not detected (proxy is stateless, has no objects).
- REST API branch deletion and spam MRs are not covered (needs OpenShell deny rules / accept as reversible).
- No caller auth or TLS on the service; one service per sandbox and ref.
- Profile `zero-gitlab-push` stays imported on the gateway.
