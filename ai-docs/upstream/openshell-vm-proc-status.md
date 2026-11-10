# Comment for NVIDIA/OpenShell#4014

**Target:** comment on https://github.com/NVIDIA/OpenShell/issues/4014

---

Still reproducible on 0.1.4 (deb, amd64). Every VM sandbox fails the same way, for any image (alpine:3.20, nvcr ubuntu 24.04 default, a custom golang:1.25-based image) and with or without a policy:

```
$ openshell -g openshell-vm sandbox create --name vm06 --from alpine:3.20 --detach
$ openshell -g openshell-vm sandbox get vm06
  Phase: Error
    - Ready: False (ProcessExited) - VM process exited with status 0
console: [0.3s] starting OpenShell VM sandbox
         Error: read /proc/self/status: Permission denied (os error 13)
```

Image-prep and the bootstrap rootfs succeed. The gateway runs as a dedicated non-root user in group `kvm` (systemd user service, `compute_driver = "vm"`, vcpus 2, mem_mib 2048).

UX gap that made this hard to debug: there is no `openshell sandbox logs`, and the guest console is only reachable inside the driver state dir of the gateway user (sudo needed). Could the console tail be shown in `sandbox get` when the phase is Error?
