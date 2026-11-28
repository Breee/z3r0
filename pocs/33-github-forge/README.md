# PoC 33: GitHub as a forge, with a GitHub App installation token (#186)

## Question

Can zero give the agent a GitHub token that is as narrow as the GitLab one:
one repository, few permissions, short life, revocable, usable for git over
https?

## Result

**Not run yet (2026-11-28).** It needs a GitHub App, and an app can only be
made in the browser by the account owner. Two scripts are ready:

1. `./setup.sh` makes a private app of your own through GitHub's manifest
   flow (one button on a local page), stores app id and private key in
   `~/.config/zero-poc33` (mode 700). Then install the app on your account
   for one or two test repositories (the link is printed).
2. `REPO=owner/name [OTHER=owner/other] ./run.sh` mints a token and checks it.

Checked without the app: both scripts parse; the JWT signature made by
`run.sh` verifies against the public key (`openssl dgst -verify`); the local
page of `setup.sh` shows the form and refuses a callback with a wrong state.

What `run.sh` checks:

| Check | Expected |
|---|---|
| the app finds its installation | 200 |
| token for one repository, permissions contents:write, issues:read, pull_requests:read, metadata:read | 201, one repository, only these permissions |
| lifetime | at most one hour |
| reads repository and issues | 200 |
| opens a pull request, reads workflow runs (not asked for) | 403 |
| second repository of the same installation | 403 or 404 |
| git clone and push to `zero/...` with user `x-access-token` | works |
| push that adds a file under `.github/workflows/` | refused (no `workflows` permission) |
| branch rules of the default branch | readable |
| `DELETE /installation/token`, then any call | 204, then 401 |

## Why this token type

Research is in #186: fine-grained personal tokens cannot be made by API;
tokens from `gh auth login` cannot be narrowed; an installation token can be
limited per call to repositories and permissions, lives one hour and can be
revoked.

## Sources

- https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app
- https://docs.github.com/en/rest/apps/installations#revoke-an-installation-access-token
- https://docs.github.com/en/apps/sharing-github-apps/registering-a-github-app-from-a-manifest
- https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
- https://docs.github.com/en/rest/repos/rules#get-rules-for-a-branch

## Limits

- Not checked here: the push gate, egress rules per path (`/graphql` is one
  path for every repository), rulesets that keep the app off the default
  branch (private repositories of a personal account need GitHub Pro), `gh`
  and the GitHub MCP server with this token, GitHub Enterprise Server.
- The token lives one hour: zero must renew it while a sandbox runs.

## Run

    ./setup.sh
    REPO=you/test-repo ./run.sh

Remove afterwards: the app under https://github.com/settings/apps, and
`rm -rf ~/.config/zero-poc33`.
