# zero roadmap

zero = the agent works freely in an isolated sandbox; host, secrets and production stay out of reach.
Two layers, kept separate:

- **Core** — the sandbox and its security guarantees. Client-agnostic.
- **Integrations** — how a client (opencode, VS Code, Claude Code, ...) uses a core sandbox. Thin adapters, no security logic.

Tracks run in parallel. Each item becomes a GitHub issue (`track:<name>`).

## Core

| Track | Now (done) | Next | Later |
|---|---|---|---|
| **Isolation** | Podman sandbox (E1): fs/proc/egress containment, attack suite | Host hardening (F15) | VM sandboxes E3 (F16, blocked upstream #4014), risk class → driver (`--vm`), in-sandbox container build (F18), K8s driver |
| **Credentials** | Placeholder secrets, per-sandbox GitLab token, revoke on rm | Egress/LLM proxy: no foreign keys (F13), token auto-renew | OpenBao-backed issuance, one-shot creds |
| **Forges** | GitLab (git, glab, MCP) | GitHub (`gh`) | other forges via `Forge` strategy |
| **Evidence & control** | — | Decision/audit log per sandbox | OTel/OCSF trace, revocation (soft/hard/quarantine), Advisor/Prover (OpenAPPA) |
| **Image** | Local uniget dev image | Signed image on ghcr.io, pulled by digest | Per-repo images / profiles |

## Integrations

Focus: **opencode and VS Code only.** Every integration uses the same core command (`zero up`) and only adds the client wiring.

| Client | Status | Next |
|---|---|---|
| **opencode** | `zero web` works; `zero attach/run` postponed (opencode #47665) | Re-enable attach/run after upstream fix |
| **VS Code** | `zero code` (Remote-SSH, Copilot) — not re-tested since Go rewrite | Re-test, one-command open |
| **Paperclip** | Evaluated | `paperclip-plugin-zero` sandbox provider (orchestration + UI) |
| Others (Claude Code, Codex, Copilot CLI) | out of scope for now | — |

## Usability (cross-cutting)

1. `zero` alone = up + open default client
2. `zero doctor` — checks host setup, prints fixes
3. `zero install` — PATH + guided host setup
4. `zero sync` — bring sandbox branches to host safely
5. Clear progress + actionable errors

## Order

Core first. Integrations start only after the core gate passes.

**Core gate** (all must pass):
1. F13 egress/LLM proxy — PoC03 7/7, attack.sh PASS
2. F15 host hardening — no docker group, gateway key off login user, breakout as login user ≠ host root
3. Signed dev image from ghcr.io, pulled by digest
4. Token lifecycle: auto-renew + revoke verified
5. Full attack suite green on current versions (OpenShell, opencode, image)
6. Portable, seamless setup — one command installs zero + gateway + host hardening on a fresh machine, idempotent, no manual sudo steps beyond one prompt; verified on a clean VM per supported OS
7. VM driver (F16) — tracked, not gating while blocked upstream (#4014); re-checked each OpenShell release

Then:
1. Usability: `zero`, `zero doctor`, `zero sync`
2. opencode + VS Code integrations
3. GitHub forge, Paperclip plugin, evidence log
4. VM driver (when unblocked), OpenBao, K8s
