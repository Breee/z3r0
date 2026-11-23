# PoC 16: forge egress limited to the workspace's projects (#52, C35)

Question: can the sandbox reach only its own projects on the forge host, per sandbox?

Result (2026-11-23, OpenShell 0.1.8, gitlab.com, by hand, no run.sh yet): yes.

Setup: `profile.yaml` (credentials only, no endpoints), one provider per sandbox,
`policy.yaml` (sandbox policy with the forge endpoint: `rules` with one allow per
method and path, `credential_binding.provider`). Token for project 88739432 only.

| Request from the sandbox | Result | Refused by |
|---|---|---|
| own project by id, by encoded path (our token) | 200 | |
| `git ls-remote` own project | works | |
| own project, method not listed (PUT) | 403 | OpenShell |
| other project (private, public), with our token, no token, a foreign token in `PRIVATE-TOKEN` | 403 | OpenShell |
| `/../`, `%2e%2e`, `//`, upper case path to another project | 403 | OpenShell |
| `/user`, `POST /snippets`, GraphQL, MCP endpoint, pipeline trigger in a foreign project | 403 | OpenShell |
| raw file and web page of a foreign public project, `git ls-remote` of another project | 403 | OpenShell |
| own project, tool beyond the token's permissions | 403 | GitLab |
| `openshell policy set` on the running sandbox: project taken out, another put in | old 403 (OpenShell), new reaches GitLab | |

Refusal body: `{"error":"policy_denied","policy":"forge","rule":"GET /api/v4/projects/278964",...}`.

Facts:
- `access` and `rules` cannot be combined; with `rules` everything else is refused.
- Rules of a profile and of the sandbox policy add up, so the shared profile must
  not carry the forge endpoint; the sandbox policy carries it.
- The path is made canonical before the match and sent in that form.
- `/api/v4/projects/1/**` does not match `/api/v4/projects/1`; both are needed.
- Headers and bodies cannot be matched. A foreign token still passes to the
  workspace's own projects (it can only do what its owner may do there).

Not tested: glab without GraphQL, opencode's MCP entry, group paths, LFS,
uploads, a self-hosted host, policy change by `zero up` on an existing sandbox.
