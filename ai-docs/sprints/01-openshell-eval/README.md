# Sprint 01 — OpenShell evaluation + IDE integration

Goal: Know whether OpenShell can host agent tool/shell execution for OpenCode and VS Code Copilot Chat, and what plugins we must write.
Scope: Linux laptop, rootless Podman driver. No OpenBao, no K8s, no Phantom middleware yet.
Exit: `F06` decision written, backed by proofs from F01–F05 and attack results F07–F09.
Order: F02 → F07 → F08 → F03 → F09 → F10 → F04 → F05 → F06 (security first).

| ID | Feature | Status |
|---|---|---|
| F01 | OpenShell local install + baseline sandbox | done |
| F02 | Containment proof (fs, process, egress) | done |
| F03 | Provider placeholder credentials | doing (PoC) |
| F04 | OpenCode ↔ OpenShell integration | done |
| F05 | VS Code Copilot Chat ↔ OpenShell integration | todo |
| F06 | Integration decision + plugin plan | todo |
| F07 | Attack-vector catalog (arch + common + creative) | done |
| F08 | Common + creative attack suite | done |
| F09 | Host compromise blast radius + hardening | done |
| F10 | Own egress/LLM gateway: strip foreign keys (C18) | deferred |
