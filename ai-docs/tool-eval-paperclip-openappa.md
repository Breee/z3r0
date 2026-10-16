# Tool eval: Paperclip, OpenAPPA (2026-10-16)

Question: can zero use them, and is there a benefit? Verdict first, evidence after.

## Verdict

| Tool | Layer | Fit | Now? |
|---|---|---|---|
| Paperclip | above zero: task/approval/budget orchestration | good — its SSH execution target + bundle-over-ssh model is what `zero up` already does | no; after Phase 2 (needs real push/MR loop first) |
| OpenAPPA | inside zero: per-tool-call data-flow guardrail | good — covers what OpenShell cannot (which data may flow to which tool); opencode adapter exists | no; API unstable (preview/RFC); revisit when Advisor/Prover (arch §7, §24.11) is scheduled |

Neither replaces anything in zero. OpenShell stays the boundary (D01). Both would be
UX/policy layers on top (D02 logic applies).

## Paperclip

- github.com/paperclipai/paperclip, MIT, TypeScript, ~103k stars, active.
- Agent-company orchestrator: org chart, tasks, approvals, review gates, budgets, heartbeat queue, activity log, encrypted secrets.
- Adapters: claude_local, codex_local, opencode_local, hermes, cursor, openclaw_gateway, http.
- docs/agents-runtime.md §9: local CLI adapters run unsandboxed on the host. Not acceptable as-is (Z1–Z6).
- Execution targets (`packages/adapter-utils/src/execution-target.ts`): `local` and `remote/ssh`; sandbox providers (sandbox-managed-runtime, file-sync, callback-bridge).
- Cross-run contract: bundle local worktree → remote run dir over ssh, no git remote inside, restore commits back; adapters never push. Same shape as zero (git bundle upload, push only via brokered creds).

Plugin system: `@paperclipai/plugin-sdk` (`packages/plugins/sdk`, spec `doc/plugins/PLUGIN_SPEC.md`, docs.paperclip.ing/administration/plugins/). JSON manifest + worker `definePlugin({ setup(ctx) })`; hooks: UI slots, events, jobs, tools, actions, local folders, DB namespace, agent sessions, and **sandbox providers** (`packages/plugins/sandbox-providers`, reference `paperclip-plugin-fake-sandbox`).

Integration path: `paperclip-plugin-zero` sandbox provider → `zero up` / ssh-config exec / bundle sync / teardown. No fork. Fallback: Paperclip `remote/ssh` target → `openshell sandbox ssh-config` host. Paperclip never holds repo creds; push stays zero's job (F12). Secrets Paperclip injects must be placeholders, not real tokens, or it breaks Z-invariants.

Benefit: approvals, budgets, audit UI, multi-agent without writing it ourselves. Candidate for zero's operator UI instead of building one (user, 2026-10-16).
Cost: Postgres + Node service, trust in its secret handling, one more identity to bind to sessions (§22).

## OpenAPPA

- github.com/archestra-ai/OpenAPPA, MIT, Rust, 1.6k stars, preview/RFC ("config and wire surfaces may break").
- Deterministic information-flow control: labels data audience×trust from event log, checks each tool call before execution, returns remedy plans (sanitize / require authority / isolate in sub-agent). TOML policy, engine does no I/O; in-process or sidecar hook.
- Crates incl. `appa-adapter-opencode`; `appa describe --check`, `appa replay` for CI.

Integration path: run as opencode plugin/sidecar inside the sandbox image (F11 dev image). Decision log exported to host as evidence (§18). Enforcement inside the sandbox is advisory from the host's view; the hard line remains OpenShell policy + supervisor.

Benefit: prompt-injection → exfil class (C18-style via tool args) gets a deterministic check before OpenShell ever sees the egress. Matches Advisor/Prover slot in arch.
Cost: unstable API, policy authoring, no proof yet that it survives a hostile agent in-sandbox (agent could disable its own plugin → must be enforced by image/policy, verify in a PoC).

## Follow-ups

- Phase 2 backlog: PoC `paperclip-plugin-zero` sandbox provider (one task end-to-end, no real creds); UI candidate.
- Advisor/Prover work: PoC "openappa in dev image, tamper test: agent tries to remove plugin".
