# Architecture Decision Records

MADR format. One file per decision: `NNNN-<slug>.md`, front matter `status`, `date`, `deciders`, `requirements` (IDs from `docs/architecture/requirements.yaml`). Append-only: supersede, never rewrite. Numbers 0018–0019 were never assigned.

| ADR | Date | Decision | Requirements |
|---|---|---|---|
| [01](0001-openshell-is-the-runtime-substrate.md) | 2026-10-09 | OpenShell is the runtime substrate | Z-RULE-30 |
| [02](0002-ide-plugins-are-ux-only-never-a-security-boundary.md) | 2026-10-09 | IDE plugins are UX only, never a security boundary | Z-RULE-17 |
| [03](0003-sprint-01-targets-linux-rootless-podman-driver-fir.md) | 2026-10-09 | Sprint 01 targets Linux + rootless Podman driver first |  |
| [04](0004-podman-6-1-3-manual-install-is-required.md) | 2026-10-09 | Podman 6.1.3 (manual install) is required |  |
| [05](0005-hard-requirement-agent-is-never-root-and-cannot-us.md) | 2026-10-09 | Hard requirement: agent is never root and cannot use host Podman | Z-OS-3 |
| [06](0006-sandbox-breakout-full-compromise-of-the-gateway-s.md) | 2026-10-09 | Sandbox breakout = full compromise of the gateway's host user (gateway, KEK + provider secrets, podman, ~/.ssh) |  |
| [07](0007-foreign-key-exfil-c18-mitigated-by-a-openshell-que.md) | 2026-10-09 | Foreign-key exfil (C18) mitigated by (A) OpenShell query allowlist and/or (B) our own egress or LLM proxy that strips client auth and injects ours |  |
| [08](0008-opencode-runs-inside-the-sandbox-option-a.md) | 2026-10-09 | OpenCode runs inside the sandbox (Option A) |  |
| [09](0009-vs-code-copilot-chat-runs-via-remote-ssh-into-the.md) | 2026-10-09 | VS Code Copilot Chat runs via Remote-SSH into the sandbox (`openshell sandbox connect N --editor vscode`), not via a host extension |  |
| [10](0010-go-openshell-0-1-2-rootless-podman-is-the-z3r0-dev.md) | 2026-10-09 | GO: OpenShell 0.1.2 + rootless Podman is the Z3R0 dev-laptop sandbox |  |
| [11](0011-f03-closed-on-the-httpbin-poc.md) | 2026-10-15 | F03 closed on the httpbin PoC |  |
| [12](0012-zero-cli-is-go-stdlib-only-cmd-zero-internal-zero.md) | 2026-10-15 | `zero` CLI is Go (stdlib only, `cmd/zero`, `internal/zero`) |  |
| [13](0013-phase-1-reopened-with-sprint-04-f16-microvm-f18-sa.md) | 2026-10-16 | Phase 1 reopened with sprint 04 (F16 MicroVM, F18 sandbox build) before Phase 2 planning | Z-P0-8, Z-P0-9, Z-OS-7 |
| [14](0014-project-name-is-zero-z3r0-zero.md) | 2026-10-16 | Project name is `zero` (z3r0 = zero |  |
| [15](0015-one-driver-per-openshell-gateway-so-zero-runs-two.md) | 2026-10-16 | One driver per OpenShell gateway, so zero runs two gateways (`openshell` = podman/E1, `openshell-vm` = vm/E3) and selects by risk class via `zero up --vm` / `Z3R0_DRIVER` / repo `.z3r0.yaml` (arch §6.2 C0–C4→E1/E3) | Z-OS-7 |
| [16](0016-paperclip-and-openappa-are-candidate-layers-above.md) | 2026-10-16 | Paperclip and OpenAPPA are candidate layers above/inside zero, not replacements: Paperclip = orchestration (tasks/approvals/budget) driving an OpenShell sandbox via its SSH execution target |  |
| [17](0017-paperclip-is-the-candidate-operator-ui-for-zero-vi.md) | 2026-10-16 | Paperclip is the candidate operator UI for zero (via a `paperclip-plugin-zero` sandbox-provider plugin), instead of building our own UI |  |
| [20](0020-zero-mints-one-fine-grained-gitlab-token-per-sandb.md) | 2026-10-16 | zero mints one fine-grained GitLab token per sandbox from the user's glab session (`POST /user/personal_access_tokens`, granular scopes on the single project, expires next day, revoked on `zero rm`), stored only as an OpenShell provider credential | Z-INV-4, Z-P0-6, Z-P0-7 |
| [21](0021-the-dev-image-is-published-to-ghcr-io-ci-built-cos.md) | 2026-11-10 | The dev image is published to ghcr.io (CI-built, cosign-signed, digest-pinned) and pulled from there |  |
| [22](0022-gitlab-endpoint-in-profile-zero-gitlab-sets-allow.md) | 2026-11-10 | GitLab endpoint in profile `zero-gitlab` sets `allow_encoded_slash: true` (C36) |  |
| [23](0023-agents-never-push-container-images.md) | 2026-11-10 | Agents never push container images | Z-P0-8, Z-RULE-8 |
| [24](0024-runtime-dependency-devops-backbone.md) | 2026-11-10 | Damage minimization on a DevOps/GitOps backbone; runtime is a swappable dependency | Z-INV-1, Z-OS-7 |
| [25](0025-control-plane-basic-and-enterprise.md) | 2026-11-10 | One zero control plane, local (basic) or central (enterprise) — proposed | Z-INV-3, Z-INV-4, Z-INV-6 |
| [26](0026-glab-bearer-placement.md) | 2026-11-11 | glab sends the GitLab token as Bearer (GLAB_IS_OAUTH2) | Z-INV-1 |
| [27](0027-no-sync-back.md) | 2026-11-11 | No sync-back: git push is the only exit | Z-INV-1 |
| [28](0028-dev-image-digest-pin.md) | 2026-11-11 | Dev image CI-signed, pinned by digest, no host signature check yet | Z-INV-5 |
