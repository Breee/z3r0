# PoC 22 — Tool lists and skills per key through LiteLLM, held against agentgateway and plain files (#172)

Question: can LiteLLM give each sandbox key its own tool list and its own skills, and how does that compare with agentgateway (PoC 15) and with plain files (#161)?

Result (2026-11-27, LiteLLM v1.110.3 with Postgres, opencode 2.2.6, OpenShell 0.1.8, Docker 29.5.2): **yes, both work per key, but only through the API and the database, not from a config file.** `run.sh`: 14 PASS, 0 FAIL, 1 NOTE.

## Sources

- Skills Hub: https://docs.litellm.ai/docs/skills_gateway, https://docs.litellm.ai/docs/skills
- Tools per key: https://docs.litellm.ai/docs/mcp_grant_access, https://docs.litellm.ai/docs/mcp_control, https://docs.litellm.ai/docs/mcp
- The request forms used are listed in the sources comment on #172.

## Chain

```
tools:  key -> LiteLLM /mcp -> MCP stand-in "notes" (its token stays in LiteLLM)
skills: admin uploads ZIP -> LiteLLM Skills Hub (database) -> key fetches archive
        -> SKILL.md placed in the sandbox -> opencode reads the folder
```

The stack is the one of PoC 17 and 18 (LiteLLM, Postgres, MCP stand-in). The prompt in the sandbox goes through zero's own model proxy, which must run.

## Result

| Check | Result |
|---|---|
| Key of the admin user with a tool list | lists that tool only, the other tool is refused |
| Key of the admin user with `no-mcp-servers` | no tool |
| Key of the admin user without any list | NOTE: every tool |
| Skill uploaded as a ZIP by the admin | 200, published at once |
| Made private (`unpublish`) | 200, the admin sees `visibility: private` |
| Key with the skill granted | skill listed, archive 200, `SKILL.md` at the root of the ZIP |
| Key without the grant | skill not listed, archive 404, content 404 |
| `SKILL.md` from the archive in a sandbox folder named in a project `opencode.json` | opencode uses the skill (`PONG-172`) |

## Findings

1. Skills exist only as database objects. They get in by upload (API or UI); no form for the config file was found in the docs. The tables were present in this database without an extra step.
2. An upload by the admin is published at once and is public until it is made private.
3. A key without the grant sees an empty list and gets 404, not 403.
4. The archive is a plain ZIP. LiteLLM does not hand skills to opencode; something must fetch the archive and put it where opencode reads. zero already copies skill folders read-only into the sandbox (#161).
5. A key of the admin user without a list sees every tool. A key zero mints must always carry a tool list or `no-mcp-servers`.

## Compared

Tool list per workspace:

| | LiteLLM | agentgateway (PoC 15) | 
|---|---|---|
| Who the caller is | the sandbox's key | the sandbox's verified identity (SPIFFE) |
| Where the list lives | on the key, in LiteLLM's database, set by API | in zero, asked per request |
| Hidden and refused | yes (run) | yes (run) |
| Needs | Postgres; already there when the model proxy is on | SPIRE and a gateway container; no database |
| Change takes effect | when the key is updated or minted anew (not run) | at the next request (run) |
| Credential in the sandbox | the proxy key, or its placeholder (PoC 20) | none |

Skills:

| | LiteLLM Skills Hub | Plain files (#161, built) |
|---|---|---|
| Source of truth | database, filled by upload | folders named in zero's config |
| Declared in a file | no | yes |
| Per workspace | grant on the key | list in the workspace |
| Reaches opencode | only if something fetches the archive | yes, read-only |
| Review of a change | LiteLLM's own approval for members | the review of the file's repository |

## Recommendation

- Skills and instructions stay plain files from zero's config. The Skills Hub adds a second store that no file describes; it can become one more source later, fetched with the sandbox's key.
- Tool lists: when the model proxy is on, the list goes on the sandbox's LiteLLM key. It is one part less to run locally, and the key and database are already there. agentgateway stays the way for a setup without LiteLLM or with workload identity.

## Limits

- One skill, one MCP stand-in with two tools, keys of one user.
- Read-only placement in the sandbox is not shown again here; the folder was a scratch folder. #161 shows it.
- Changing a key's tool list or grant after it was minted was not run.
- A member's upload and its approval were not run.
- Docker only (ADR 0051).

## Run

```
pocs/22-litellm-tools-skills/run.sh      # needs the state of PoC 17, a zero sandbox with the model proxy, jq, zip
```
