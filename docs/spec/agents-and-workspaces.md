# Spec: agents and workspaces in one config file

Status: design (#157, ADR 0049). Built: `folders`, `tools` (#132), `instructions`, `skills` (#161), `parts.models`, `providers`, `models` per workspace (#162). Everything else on this page is not built.

## 1. The file

`~/.config/zero/config.yaml`. No credential is ever written here. The example is the target shape: today's zero reads only `current`, `folders`, `tools`, `instructions` and `skills` (the last two per workspace, where they stay valid) and refuses the other fields.

```yaml
current: payments

parts:                       # opt-in containers behind zero-cp; none is needed for the core
  models: litellm            # model proxy and guardrails
  tools: agentgateway        # MCP gateway (ADR 0043)
  secrets: keyring           # or: openbao

providers:                   # which model providers the model proxy offers
  github-copilot:
    login: per-user          # the person connects once; the login stays in the proxy
  anthropic:
    key: secret://anthropic  # a name in the secret store, not the key

mcp:                         # MCP servers the tool gateway may reach
  gitlab:
    url: forge               # the instance endpoint of the workspace's forge host
  docs:
    url: https://mcp.example.com/mcp
    token: secret://docs-mcp

guardrails:                  # applied by the model proxy to every call
  - mask: [email]
  - block: [forbiddenmarker]

agents:
  reviewer:
    model: github-copilot/claude-sonnet-5.5
    instructions: [~/zero/instructions/review.md]
    skills: [~/zero/skills/go-review]
    mcp:
      gitlab: [get_merge_request, list_commits]
    capabilities: [git.read, gitlab.mr.read]
  builder:
    preset: gitlab-developer
    model: github-copilot/claude-sonnet-5.5

workspaces:
  payments:
    folders:
      - ~/src/payments
      - ~/src/payments-specs
    agents: [reviewer, builder]
    network: [proxy.golang.org]
```

Short forms stay valid: a workspace with only `folders` (and today's `tools`, `instructions`, `skills`) behaves as it does now, with one unnamed agent of preset `gitlab-developer`.

## 2. How each field is applied

| Field | Rendered into | Enforced by (outside the sandbox) | State |
|---|---|---|---|
| `folders` | checkout or read-only copy in the sandbox | sandbox, push gate | built |
| `capabilities`, `preset` | permissions of the forge token, gate settings | forge, push gate | built (capabilities come from `.zero/policy.yaml` today) |
| `network` | rule `project` of the sandbox policy | OpenShell | built (from `.zero/policy.yaml` today) |
| `instructions`, `skills` | read-only folder `/tmp/zero-input/.zero`, opencode user config | read-only mount | built for opencode (#161) |
| `model`, `providers` | LiteLLM `model_list`, `credential_list`; one key per sandbox with the agent's models; model list in the harness config | model proxy | PoC 17, 18 |
| `guardrails` | LiteLLM `guardrails` | model proxy | PoC 18 |
| `mcp` | with the model proxy: MCP servers in LiteLLM, tool list on the sandbox's key; else agentgateway targets, tool list per sandbox identity from zero-cp per request | tool gateway | PoC 22, PoC 15 |
| `secret://NAME` | lookup at the part that needs it | secret store | PoC 10, 12 (OpenBao) |

A sandbox of a workspace with models gets no Copilot login (#162, PoC 23). Its key for the proxy stays outside too: the sandbox holds a placeholder that is swapped only on the way to the proxy (#162, PoC 20). Shown: a key minted with `no-mcp-servers` sees no tool; the key can be a placeholder (PoC 20); the Copilot CLI needs no GitHub login behind the proxy (PoC 23); opencode finds no model of the proxy by itself, so zero writes the list, with the token limits of each model for which the proxy reports both, and without the models the proxy marks as not for chat (embedding and the like) (PoC 21).

zero-cp decides and hands out; the traffic itself goes from the sandbox to the part (model proxy, tool gateway) under an egress rule, as in PoC 15 and 18, not through zero-cp.

Rules: layers only narrow (operator limits, shared definitions, repository `.zero/policy.yaml`, this file); a wider setting asks for approval as today. `zero up` prints what each agent gets.

## 3. One choice per job

| Job | Choice | Reason |
|---|---|---|
| Models, guardrails | LiteLLM, opt-in | discovery, per-key model list, mask and block shown (PoC 18). Cost: Postgres. The key stays outside the sandbox, the sandbox holds a placeholder (PoC 20, built in #162) |
| Tool list per workspace | on the sandbox's LiteLLM key when the model proxy is on; else agentgateway | with the proxy on, the key and its database are already there and the list is set when zero mints the key; a key zero mints always carries a list or `no-mcp-servers`, because a key of the admin user without one sees every tool (PoC 22). agentgateway needs no database and takes the identity of the sandbox instead of a key, but needs SPIRE (PoC 15); it stays the way without LiteLLM |
| Skills, instructions | plain files, read-only folder | no part needed and the file is the source of truth. LiteLLM keeps skills in its database, grants them per key and hands out a ZIP (PoC 22), but has no file form for them, does not deliver to opencode and has nothing for instructions |
| Secrets | host keyring; OpenBao opt-in | as today |

Sources: `ai-docs/tool-eval-litellm.md`, `pocs/15-gitlab-mcp`, `pocs/17-litellm`, `pocs/18-litellm-sandbox`.

## 4. Packaging

One compose file shipped with zero, one profile per part (`models`, `tools`, `secrets`); images pinned by digest; every port bound to 127.0.0.1. `zero up` starts the profiles named under `parts` with `docker compose` (ADR 0051; `ZERO_COMPOSE` names another compose command) and renders their config from this file into zero's state directory. zero-cp itself stays the user unit it is today.

Decided (ADR 0051): the parts run under Docker until the podman on the host runs container health checks (C58, #174); zero's sandboxes stay on rootless podman. `podman compose down` also fails on Ubuntu 26.04 (C57).

## 5. First UI (read-only)

Built in #163 (ADR 0053): `zero ui`, port 18097, pages Workspaces, Workspace (with the gate's decisions), Parts. The model proxy's own page is linked, not passed on, and keeps its own login (C59).

Served by zero-cp under `/ui/`, data from `/api/` (JSON), which the CLI and a terminal UI use too. It gets its own listener on 127.0.0.1 with a port of its own: the gate port is reachable from every sandbox (`host.openshell.internal:18095`), so nothing but the gate may answer there, and no sandbox policy names the UI port.

| Page | Shows | Read from |
|---|---|---|
| Workspaces | name, status, folders, token time left, policy state | what `zero ls` reads |
| Workspace | agents, what each may reach (hosts, capabilities, tools, model, instructions, skills) | config file, state directory |
| Decisions | pushes allowed, denied, refused requests | `gate.log` |
| Parts | which are on, health | compose state |

Protection: the threat is the agent in a sandbox and a web page in the user's browser. Own loopback listener (above); every call needs a session started with a one-time link printed by `zero ui` (as `zero web` does for opencode); `Host` and `Origin` checked against rebinding and forged requests; no call that changes anything. Answers carry no token, key or gate key: token ID and expiry only, and decision lines as the gate writes them (it logs no credential).

## 6. Open

- Build steps: #161 (done), #162 model path, #163 API and read-only UI (done), #164 tool path, #165 editing in the UI.
- Agents with different rights in one sandbox share files, network and token. Until decided: one sandbox per set of rights.
- Shared definitions from a git repository pinned by commit; operator limits file.
- How Claude Code, VS Code and Copilot CLI are told about instructions and skills.
