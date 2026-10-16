# F18 — Sandbox-scoped container build
Status: todo
Description: Prove an agent can build an OCI image inside the sandbox with no host engine socket, image storage scoped to the sandbox, and a hostile Dockerfile unable to reach host secrets or arbitrary egress (arch §9.5, §25 items 8–9).
Goal: `go build` → image → push to allowlisted registry, all from inside the sandbox; hostile Dockerfile cases all blocked.
Plan:
1. Baseline in E1 (podman driver): try `buildah bud --isolation chroot`, `podman build` (rootless, vfs) in sandbox → verify: expected FAIL (userns blocked, no caps); record exact errors.
2. Namespace-free path in E1: `ko build` (Go, no Dockerfile) and `apko` → verify: image tar produced, no socket, storage under `/sandbox`.
3. Dockerfile path in E3 (needs F16): podman/buildah as VM-root, storage on overlay disk, registry pulls via supervisor-allowlisted endpoint → verify: `tests/fixtures/hello-go` Dockerfile builds; image never appears in host `podman images` (via `scripts/podman-gw.sh`).
4. Hostile Dockerfile set (arch §13.4–13.7, F07 catalog): `RUN cat ~/.ssh/*`, `--mount=type=secret`, `RUN curl evil`, `--network=host`, `--privileged` runner, `RUN mount` → verify: every case blocked or inert; add to `tests/containment/`.
5. Push built image to allowlisted registry with placeholder cred → verify: push OK, raw token never in sandbox (`env`, image layers grep).
DONE: step 1 errors logged; steps 2–5 pass/fail table in notes.md; feature matrix row added `In-sandbox container build`; caveats for anything inert-only.
GOOD: one script `pocs/06-sandbox-build/run.sh` with pass/fail table; build time measured E1 (ko) vs E3 (podman).
Proof: `pocs/06-sandbox-build/run.sh` → all PASS; host `podman images` unchanged.
Log:
