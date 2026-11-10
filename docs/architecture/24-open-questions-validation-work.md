## 24. Open questions / validation work

1. **OpenShell developer UX** — prototype OpenCode + VS Code attached to an OpenShell Podman sandbox with isolated workspace/worktree.
2. **OpenShell security fit** — validate filesystem/process/network controls against the Z3R0 corrupted-agent threat model and document platform-specific gaps (Linux/macOS/Windows).
3. **OpenShell Provider ↔ OpenBao** — define the enterprise credential-driver/profile path so real credentials remain behind the supervisor/credential authority.
4. **Phantom Proxy middleware** — implement structured Kubernetes/log redaction, connection-string parsing, cumulative disclosure tracking and organization-specific detectors as OpenShell supervisor middleware.
5. **GitLab provider/adapter** — support Git HTTPS/API, `glab`, feature-branch push, pipeline reads, MR creation and protected-branch denial using OpenShell + GitLab-native controls.
6. **Kubernetes adapter** — map `k8s.logs.read`, resource reads and DEV/QA mutations onto OpenShell network/provider policy + native Kubernetes RBAC/admission.
7. **Production diagnostic path** — prove one-shot approved PROD logs/spec reads can be fetched remotely and sanitized before data reaches the endpoint sandbox.
8. **Container workflow** — validate container build/run inside OpenShell without exposing the developer Docker socket, including hostile Dockerfiles, network controls and internal registries.
9. **MicroVM threshold** — define when Z3R0 automatically selects the OpenShell MicroVM driver instead of rootless Podman.
10. **Kubernetes compute driver** — validate enterprise shared/remote sandbox operation and interaction with Cilium/Kyverno/Tetragon.
11. **OpenShell Policy Advisor/Prover integration** — determine how Z3R0 semantic permission requests compile into low-level OpenShell deltas and how proof results appear in approval UX.
12. **SPIFFE-backed Git signing** — prototype `gpg.format=x509` signing using the active security-epoch identity and verify in GitLab/CI without exposing private signing material to the agent.
13. **Security-epoch identity** — reconcile Z3R0 sender-constrained session tokens with OpenShell sandbox/workload identity and SPIFFE/SPIRE support.
14. **LiteLLM integration** — compile session capabilities into MCP/tool visibility while keeping all tool output untrusted.
15. **Organization Context ingestion** — service-map schema, ownership, dependency fan-out, criticality and data classification; context may only escalate risk.
16. **Risk engine** — deterministic baseline first; invoke optional cheap/private model only when context could materially escalate an ambiguous action.
17. **Approval UX** — local and central trusted UI, agent-provided reason/data exposure/blast radius, one-shot grant consumption and audit.
18. **Revocation drills** — soft revoke, hard revoke and quarantine across Z3R0 + OpenShell; measure actual time-to-cut-authority.
19. **Workspace semantics** — Z3R0 workspace = set of repos; avoid confusion with any OpenShell administrative workspace concept.
20. **Dependency mirror injection** — org declares Artifactory/Nexus/GitLab/OCI mirrors; Z3R0/OpenShell injects GOPROXY/npm/pip/OCI/OS config.
21. **Optional public-content scanner** — trusted local/private model hooks; scanner may only increase suspicion/restriction.
22. **Platform prerequisite baseline** — define separately; Z3R0 will not become a remediation layer for fundamentally unsafe DevOps platforms.
23. **OpenShell maturity/support matrix** — track version compatibility, required kernel/runtime features, Kubernetes driver maturity and release cadence before enterprise GA.

---

