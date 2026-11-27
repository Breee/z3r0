---
status: accepted
date: 2026-11-27
deciders: bree
requirements: [Z-INV-1, Z-INV-3, Z-INV-4]
---

# 0053 — zero-cp serves a read-only API and UI on a loopback port of their own; the model proxy's page is linked, not passed on

## Context

ADR 0049 wants one place to see every workspace and what its agent may reach. zero-cp's gate port (18095) is reachable from every sandbox, so nothing but the gate may answer there. PoC 25 showed LiteLLM's page works below a sub-path behind a front port; LiteLLM's docs give no open-source way to sign a browser in from such a front (C59). #163.

## Decision

1. zero-cp listens on a second port, `127.0.0.1:18097`, for `/api/` (JSON) and `/ui/` (pages). No sandbox policy names this port.
2. It only reads: GET and HEAD, nothing else. The Host name must be this address, an Origin header must match it.
3. A session starts with `zero ui`: a code in zero's state folder, good once and for 5 minutes, turned into a cookie (HttpOnly, SameSite=Strict, 12 hours, kept in zero-cp's memory). Without it every call is refused.
4. No answer holds a credential: no token id or value, no gate key, no proxy key. Text that comes from a sandbox is cleaned as in the terminal.
5. Pages: Workspaces, Workspace (folders, projects, capabilities, hosts, tools, models, instructions, skills, branch, the gate's decisions), Parts. They are rendered by zero-cp, with no script and no file from elsewhere.
6. The model proxy's own page is linked (`http://127.0.0.1:18096/ui/`), not passed on through this port, and keeps its own login. Reasons: zero's proxy runs without a sub-path, and moving it below one changes the address every sandbox uses; a signed-in pass-through is not possible without a licence (C59).

## Consequences

- Any program of the login user on this machine can run `zero ui` and read the pages; they show what `zero ls` and `zero status` show. A browser page from elsewhere cannot (Host, Origin, SameSite).
- A restart of zero-cp ends every session; `zero ui` gives a new one.
- The user types the LiteLLM login on its page until C59 is decided (licence, zero-cp as OpenID provider for LiteLLM, or as is).
- Editing (#165) needs its own decision: this port then no longer only reads.
- `/api/` is what a terminal UI would use; the CLI still reads the same data directly.
