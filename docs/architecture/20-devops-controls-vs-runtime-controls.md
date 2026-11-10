## 20. DevOps controls vs runtime controls

Some attacks are best prevented through established DevOps practice; others require runtime containment.

### DevOps controls are primary for

- malicious/incorrect source-code changes;
- bad GitOps manifests;
- dependency changes;
- production promotion;
- protected branches;
- artifact provenance;
- CI validation;
- review/approval workflows.

### Runtime controls are primary for

- credential theft;
- secret exfiltration;
- architecture/source exfiltration to unauthorized destinations;
- arbitrary filesystem access;
- malicious tests/build scripts;
- malicious Dockerfiles;
- prompt-injected destructive CLI/API actions;
- direct production access outside approved workflows;
- compromised tool/plugin behavior;
- compromised workstation blast radius.

Neither class replaces the other.

Z3R0 should not replace proven platform controls. It should compose with them and close the agent-specific gaps. A mature GitOps/CI/RBAC/admission setup should become **more valuable**, not redundant, when Z3R0 is introduced.

---

