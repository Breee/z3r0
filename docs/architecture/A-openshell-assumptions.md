## Appendix A — OpenShell assumptions used by this draft

This draft assumes the following current OpenShell capabilities and treats them as dependencies to validate during implementation:

- sandbox/supervisor split with the trusted supervisor outside the agent workload; [Z-OS-1]
- Landlock-based filesystem restrictions on supported Linux hosts; [Z-OS-2]
- non-root/no-capability/no-new-privileges process posture and seccomp restrictions; [Z-OS-3]
- supervisor-mediated network policy with destination/binary/L7-aware controls; [Z-OS-4]
- provider profiles with placeholder credentials and endpoint-bound substitution; [Z-OS-5]
- supervisor middleware capable of inspecting/modifying network traffic; [Z-OS-6]
- Podman, MicroVM and Kubernetes compute drivers; [Z-OS-7]
- policy updates for dynamic network/provider controls; [Z-OS-8]
- OCSF-oriented runtime/security observability; [Z-OS-9]
- Policy Advisor/Policy Prover functionality where available. [Z-OS-10]

These are implementation dependencies, not Z3R0-owned features. Z3R0 must track OpenShell version/support changes rather than copy these mechanisms into its own codebase.

### Official references

- OpenShell: https://github.com/NVIDIA/OpenShell
- Developer Guide: https://docs.nvidia.com/openshell/home/
- Sandbox runtimes: https://docs.nvidia.com/openshell/latest/how-it-works/sandboxes/runtimes
- Security best practices: https://github.com/NVIDIA/OpenShell/blob/main/docs/security/best-practices.mdx
- Sandbox architecture: https://github.com/NVIDIA/OpenShell/blob/main/architecture/sandbox.md
- Security policy architecture: https://github.com/NVIDIA/OpenShell/blob/main/architecture/security-policy.md
