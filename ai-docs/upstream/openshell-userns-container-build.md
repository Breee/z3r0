# Comment for NVIDIA/OpenShell#4098 (related #5370)

**What:** Container builds (rootless buildah/podman, BuildKit) cannot run in any sandbox, including the VM driver. Same root cause as this issue: workload seccomp denies `CLONE_NEWUSER`.

**Why it matters:** With the VM driver the guest kernel is already the isolation boundary. Agents need to build and test container images (Dockerfile) without a host engine socket. Today only namespace-free builders (ko, apko) work.

**Repro (0.1.8, vm driver):**
```
openshell -g <vm-gateway> sandbox create --name b1 --from alpine:3.20
openshell sandbox exec -n b1 -- unshare -U -r id
# unshare: unshare(0x10000000): Operation not permitted
```
`CapEff=0, NoNewPrivs=1, Seccomp=2`, `/proc/sys/user/max_user_namespaces=8095`.

**Ask:** +1 for an administrator opt-in (gateway or per-sandbox) that allows `CLONE_NEWUSER`, at least for the VM driver.
