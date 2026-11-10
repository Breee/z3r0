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

Every integration uses the same core command (`zero up`) and only adds the client wiring.

| Client | Status | Next |
|---|---|---|
| **opencode** | `zero web` works; `zero attach/run` postponed (opencode #47665) | Re-enable attach/run after upstream fix |
| **VS Code** | `zero code` (Remote-SSH, Copilot) — not re-tested since Go rewrite | Re-test, one-command open |
| **Claude Code** | — | Run inside sandbox, attach via terminal |
| **Paperclip** | Evaluated | `paperclip-plugin-zero` sandbox provider (orchestration + UI) |
| Others (Codex, Copilot CLI) | — | On demand |

## Usability (cross-cutting)

1. `zero` alone = up + open default client
2. `zero doctor` — checks host setup, prints fixes
3. `zero install` — PATH + guided host setup
4. `zero sync` — bring sandbox branches to host safely
5. Clear progress + actionable errors

## Order

1. Usability 1–2, F13, F15
2. Signed image, GitHub forge, VS Code re-test, Claude Code
3. Paperclip plugin, evidence log
4. VM driver (when upstream fixed), OpenBao, K8s
