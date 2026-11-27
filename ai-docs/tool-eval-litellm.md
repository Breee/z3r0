# Tool evaluation: LiteLLM, every feature area held against what zero needs (#159)

State 2026-11-26. Sources: docs.litellm.ai as of that day (paths below are under `https://docs.litellm.ai`), plus two runs with LiteLLM v1.110.3: `pocs/17-litellm` (#156) and `pocs/18-litellm-sandbox` (#158). Each row says **run** (seen in one of the two PoCs) or **read** (documentation only). "Not found" means the docs searched do not say it; it does not mean impossible.

Columns: **Where** = `config` (declared in `config.yaml`), `API/UI` (only through the API or the web UI, kept in the database). **DB** = needs Postgres. **Ent.** = needs the Enterprise licence key.

## Verdict

| Job | Fit | Now? |
|---|---|---|
| Models: one address, provider logins outside the sandbox, model list per key | yes | probe done (PoC 18); build step open |
| Guardrails on model and tool calls (mask, block) | yes | probe done (PoC 18) |
| MCP servers, tool and parameter filters per server | yes, in config | probe done (PoC 18) |
| Tool list per workspace | yes when the model proxy is on: per key, database; a key without a list sees every tool when it belongs to the admin user, so zero always sends a list or `no-mcp-servers` (PoC 22) | decided (ADR 0049, amended): on the key with the proxy on, else agentgateway |
| Skills | partly: database objects per key, no config form, archive is a ZIP someone must fetch for opencode (PoC 22) | not used; plain files |
| Instructions (AGENTS.md and the like) | no: nothing delivers files or forces a system prompt per key (not found) | zero's job |
| Identity of a sandbox | partly: virtual key, or a `custom_auth` hook; JWT auth is Enterprise | probe worth running |
| Audit of each call | partly: spend logs yes; audit log of changes is Enterprise | not looked at in a run |

LiteLLM can enforce **models, tools and guardrails**. It does not replace anything in zero's core (sandbox, egress rules, push gate, forge token), and it does not deliver instructions.

## Feature map

### Models, routing, provider logins

| Feature | Where | DB | Ent. | Seen | Source |
|---|---|---|---|---|---|
| `model_list`, wildcard `provider/*` | config | no | no | run | /docs/wildcard_routing |
| Model discovery (`check_provider_endpoint`) | config | no | no | run (68 Copilot models) | /docs/proxy/model_discovery |
| OpenAI form and Anthropic form (`/v1/messages`) on one proxy | - | no | no | run | /docs/providers/github_copilot |
| Fallbacks, load balancing | config | no | no | read | /docs/proxy/configs |
| `credential_list`, referenced by `litellm_credential_name` | config | no | no | run | /docs/proxy/configs |
| GitHub Copilot, per-user OAuth, declared in config (`github_copilot_auth_type: per_user_oauth`) | config + connect in UI | yes | no | run | /docs/providers/github_copilot (documents the config form for GitHub App auth only) |
| Copilot only for keys of the LiteLLM user who connected | - | yes | no | run | /docs/providers/github_copilot |
| Secret managers (Vault and others) as key source | config | no | yes | read | /docs/secret |

### Keys, users, teams, budgets

| Feature | Where | DB | Ent. | Seen | Source |
|---|---|---|---|---|---|
| Virtual key with `models`, `duration`; delete (`metadata`: read, not run) | API/UI | yes | no | run | /docs/proxy/virtual_keys |
| Limits for new keys (`upperbound_key_generate_params`, `default_key_generate_params`, `key_alias_pattern`) | config | yes | no | read | /docs/proxy/virtual_keys |
| Key regenerate (`/key/{key}/regenerate`) | API | yes | no | read | /docs/proxy/virtual_keys |
| Service account keys (team, no user) | API | yes | no | read | /docs/proxy/service_accounts |
| Master key rotation | API | yes | no | read | /docs/proxy/master_key_rotations |
| Automatic rotation of virtual keys | - | - | - | not found on the key page | /docs/proxy/virtual_keys |
| Budgets, rate limits per key, team, user | API/UI | yes | no | read | /docs/proxy/users |
| Without database: master key only, no virtual keys, budgets silently not enforced | - | - | - | run (no keys) | /docs/proxy/docker_quick_start |

### MCP gateway

| Feature | Where | DB | Ent. | Seen | Source |
|---|---|---|---|---|---|
| Servers (`mcp_servers:`), transports http, sse, stdio | config (or API/UI) | no | no | run (http) | /docs/mcp |
| Upstream credential kept in the proxy (`auth_type`, `auth_value`, `static_headers`) | config | no | no | run | /docs/mcp_config_reference |
| OAuth upstream: per user, client credentials, pass-through, token exchange (RFC 8693) | config | yes for tokens | no | read | /docs/mcp_oauth, /docs/mcp_oauth_passthrough, /docs/mcp_obo_auth |
| Signed JWT per tool call for the upstream (`mcp_jwt_signer`) | config | no | not stated | read | /docs/mcp_zero_trust |
| Tool and parameter filter per server (`allowed_tools`, `disallowed_tools`, `allowed_params`) | config | no | no | read | /docs/mcp_control |
| Tool list per key, team, agent (`object_permission.mcp_tool_permissions`), intersected | API/UI | yes | no | run (per key); read (team and agent intersection) | /docs/mcp_grant_access, /docs/mcp_control |
| With `require_key_mcp_access_defined` a key without a list gets no server (an empty team list still does not restrict; access groups can grant) | config | yes | no | run: holds for keys without user and normal users, **not for admin-user keys** | /docs/mcp_control |
| Access groups | config + API | yes | no | read | /docs/mcp_control |
| Toolsets | API/UI | yes | no | read | /docs/mcp_toolsets |
| Tool list pinning (freeze names, descriptions, schemas) | API | yes | no | read | /docs/mcp_control |
| Guardrails on tool calls and results | config | no | no | run (block on a call) | /docs/mcp_guardrail |
| Tool list pinning (tools, descriptions, schemas frozen) | API | yes | not stated | read | /docs/mcp_control |
| Cost and rate limit per server | config/API | partly | no | read | /docs/mcp_cost |
| Tool search (few virtual tools instead of the full list) | config + API | yes | no | read | /docs/mcp_tool_search |
| OpenAPI file turned into an MCP server | API/UI | yes | no | read | /docs/mcp_openapi |

### Skills, prompts, agents

| Feature | Where | DB | Ent. | Seen | Source |
|---|---|---|---|---|---|
| Skills Hub: ZIP upload, import from GitHub or by URL, versions, review | API/UI | yes | not stated | run (ZIP upload, unpublish); read (rest) | /docs/skills_gateway |
| Skills per key or team (`object_permission.skills`) | API/UI | yes | not stated | run (per key, PoC 22); read (team) | /docs/skills_gateway |
| Delivery: Claude Code plugin marketplace, Codex index, archive download API | - | yes | not stated | run (archive, PoC 22); read (rest) | /docs/tutorials/claude_code_plugin_marketplace, /docs/skills_gateway |
| Skill scan before publishing (`skill_scanner`) | config | yes | not stated | read | /docs/skills_gateway |
| Skills declared in `config.yaml` | - | - | - | not found (page read: no config form) | /docs/skills_gateway |
| Skills for opencode | - | - | - | not found | /docs/tutorials/opencode_integration |
| Prompts (`prompts:`, dotprompt files, GitLab as source); client must name `prompt_id` | config | no | no | read | /docs/proxy/prompt_management |
| System prompt forced per key or team | - | - | - | not found (page read) | /docs/proxy/prompt_management |
| Instruction files delivered to an agent | - | - | - | not found (page read) | /docs/agent_resources |
| Agents (`agents:`, A2A route), key bound to an agent gets the intersection of tool lists | config (or API/UI) | yes for keys | not stated | read | /docs/a2a, /docs/a2a_agent_permissions |
| Iteration and session budgets, kill switch for agents | API | yes | not stated | read | /docs/a2a_iteration_budgets |

LiteLLM's "agent" is a remote agent service reached through the proxy, not a coding tool with instructions and skills. It overlaps with zero's planned agent definition only in the tool list.

### Guardrails and policies

| Feature | Where | DB | Ent. | Seen | Source |
|---|---|---|---|---|---|
| Built-in content filter (patterns, word lists, mask or block), no outside service | config | no | no | run | /docs/proxy/guardrails/litellm_content_filter |
| Presidio for free-text personal data (own two containers) | config | no | no | read | /docs/proxy/guardrails/pii_masking_v2 |
| Secret detection | config | no | yes | read | /docs/proxy/guardrails/secret_detection |
| Prompt injection check, judge by a model | config | no | no | read | /docs/proxy/guardrails/quick_start |
| Tool permission guardrail (rules on tool names and arguments in model calls) | config | no | no | read | /docs/proxy/guardrails/tool_permission |
| Own guardrail in Python | config | no | no | read | /docs/proxy/guardrails/custom_guardrail |
| Policies: global or per model in config; attached to a team or key | config / API | yes | attach: yes | read | /docs/proxy/guardrails/guardrail_policies |
| Routing of sensitive requests to a local model | config | no | not stated | read | /docs/proxy/guardrails/quick_start |
| About 40 vendor guardrails, each needs the vendor's service | config | no | varies | read | /docs/proxy/guardrails/quick_start |

### Logging, audit, data protection

| Feature | Where | DB | Ent. | Seen | Source |
|---|---|---|---|---|---|
| Spend log per call (key, model, tokens); prompts not stored by default | config | yes | no | read | /docs/proxy/ui_logs |
| Message logging off, redaction headers, retention period | config | yes | no | read | /docs/proxy/spend_logs_deletion |
| Callbacks (OpenTelemetry, Prometheus, Langfuse, S3 ...) | config | no | no | read | /docs/proxy/logging |
| Audit log of changes to keys, teams, models, guardrails | config | yes | yes | read | /docs/proxy/audit_logs |
| Offline mode: stops the outbound requests listed on that page, each with its own switch (`LITELLM_OFFLINE_MODE`) | env | no | no | read | /docs/proxy/outbound_requests |
| Delete everything of one person | - | - | - | not found (pages read) | /docs/proxy/spend_logs_deletion, /docs/proxy/ui_logs |

### Identity, UI, deployment, licence

| Feature | Where | DB | Ent. | Seen | Source |
|---|---|---|---|---|---|
| Own check of the caller in Python (`custom_auth`) | config | no | no | read | /docs/proxy/custom_auth |
| JWT auth (issuer, claims to team and user) | config | yes | yes | read | /docs/proxy/jwt_user_team_auth |
| Sign-in to the UI with OIDC (free up to 5 users) | config | yes | above 5: yes | read | /docs/proxy/admin_ui_sso |
| SAML | config | yes | yes | read | /docs/proxy/authentication |
| SCIM | config | yes | yes | read | /docs/tutorials/scim_litellm |
| Client certificate checked in front of the proxy, name passed in a header | ingress | no | no | read | /docs/proxy/deploy/mtls |
| Web UI (all pages) | - | yes | no | run (login, credentials) | /docs/proxy/ui |
| Pass-through endpoints; allow or deny per key | config / API | yes | per key: yes | read | /docs/proxy/pass_through |
| One image, 1.36 GB, about 620 MiB idle, 780 MiB with Postgres, 17 to 36 s start | - | - | - | run | pocs/17-litellm |
| Helm chart; several replicas need Postgres and Redis | - | yes | no | read | /docs/proxy/prod |
| Images signed with cosign: with a key since v1.83.0, keyless since v1.111.0-dev.3; SBOM not found | - | - | - | read | /docs/proxy/docker_image_security |
| Compromised releases in 2026: v1.107.0 (October); 1.82.7/1.82.8 (March) | - | - | - | read | /blog/security-incident-oct-21-2026, /blog/security-update-march-2026 |
| Open source (MIT, per the licence file of the repository); Enterprise is a licence key on the same image | - | - | - | read | /docs/enterprise, https://github.com/BerriAI/litellm/blob/main/LICENSE |

## zero's needs

| zero need | LiteLLM feature | Fit | Cost |
|---|---|---|---|
| Agent definition (tool, model, instructions, skills, tools) | none that matches; `agents:` is something else | no | zero's own config file |
| Model per agent | `models` list on the key; default model is the client's setting | yes | database; zero renders the client config |
| Model list in opencode | `/v1/models` of the key | yes | zero renders it at `zero up` (PoC 18) |
| Skills | Skills Hub, per key | partly | database, not declarative; zero would fetch the archive for opencode |
| Instructions | nothing | no | read-only folder by zero |
| MCP servers | `mcp_servers:` | yes | none beyond the proxy |
| Tool list per workspace | `object_permission` on the key | partly | database; key in the sandbox; admin-user keys fail open |
| Credentials outside the sandbox | provider logins and MCP tokens stay in the proxy | yes | the LiteLLM key can be a placeholder for clients that send it as Bearer (PoC 20, #170); zero's code still puts the key in the sandbox (#162) |
| Identity of a sandbox | virtual key per sandbox; `custom_auth` could take zero's identity | partly | key mint and revoke by zero; hook is own Python code in the proxy |
| Guardrails, DSGVO | content filter, Presidio | partly | pattern data yes; free text needs Presidio; forge traffic does not pass LiteLLM |
| Audit per call | spend logs, callbacks | partly | database; change audit is Enterprise |
| Revoke at pause and rm | `/key/delete` | yes | zero must call it (PoC 18 by hand) |
| Operator limits | `upperbound_key_generate_params`, filters per server, global guardrails | partly | per team or key policies are Enterprise |
| One UI | LiteLLM's UI for its own objects | partly | not zero's workspaces, policy or decision log |

## What stays zero's job in every case

The sandbox, its egress rules, the push gate, the forge token and its permissions, policy approval, the workspace config, instructions, and the rule that the sandbox reaches only the proxy. LiteLLM sees model and tool calls, nothing else.

## Recommendation per job

| Job | Recommendation | Reason |
|---|---|---|
| Models | LiteLLM, opt-in | shown end to end; removes provider logins from the sandbox once the Copilot login is dropped there |
| Guardrails | LiteLLM, with the models | declared in config, no extra service for patterns |
| MCP servers and tool lists | open | LiteLLM: one place and UI, but database, key in the sandbox, fail-open found. agentgateway (ADR 0043): no database, identity verified, list per request from zero-cp, no UI |
| Skills, instructions | plain files, read-only folder from zero's config | LiteLLM has no declared form and none for opencode; instructions not at all |
| Agent definition, operator limits | zero's own config | no LiteLLM feature matches |

## Probes worth running

1. Sandbox without its own Copilot login; model egress only to the proxy (not shown in PoC 18).
2. LiteLLM key held outside the sandbox as an OpenShell placeholder. Done: works (PoC 20, #170).
3. Copilot connected by a LiteLLM user who is not admin; or service account keys; does the fail-open for tool lists go away.
4. `custom_auth` hook that accepts the sandbox identity (JWT-SVID) in place of a stored key; works without database?
5. Skills: one skill, two keys, archive fetched into a sandbox, picked up by opencode. Done: PoC 22 (#172).
6. Presidio for free-text personal data.

## Corrections to earlier statements

- "The per-user Copilot credential cannot be declared in the config file" (said in the session on 2026-11-25): wrong. `credential_list` with `github_copilot_auth_type: per_user_oauth` works on v1.110.3 (`pocs/17-litellm/config.yaml`).
- "LiteLLM does nothing for skills" (said on 2026-11-26): wrong. It has a Skills Hub; see above for its limits.
- The first failed runs of PoC 17 were a mistake in `run.sh`, not a result of C57 (corrected in #156).
