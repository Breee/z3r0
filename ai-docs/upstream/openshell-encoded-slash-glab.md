# Comment for NVIDIA/OpenShell#4671

**Target:** comment on existing issue https://github.com/NVIDIA/OpenShell/issues/4671 (same root cause, adds an impact case).

---

Another real-world impact: the GitLab CLI (`glab`) encodes the project path in the request-target for almost every command, e.g. `GET /api/v4/projects/group%2Fsub%2Frepo/merge_requests`. In a sandbox with a GitLab provider credential on `gitlab.com:443` (`protocol: rest`, `enforcement: enforce`):

```
$ glab mr list
ERROR Get "https://gitlab.com/api/v4/projects/corewire%2Ftechnology-and-research%2Fdeveloper/merge_requests?page=1&per_page=30&state=all": EOF.
$ curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $GITLAB_TOKEN" https://gitlab.com/api/v4/projects/corewire%2Ftechnology-and-research%2Fdeveloper
000   (curl exit 56)
$ curl ... https://gitlab.com/api/v4/projects/88739432
200
```

Both glab and the agent see only `EOF`, so neither can tell that the policy blocked it, and nothing hints at `allow_encoded_slash`.

Two asks:
1. As in this issue: return a structured `403 policy_denied` naming `allow_encoded_slash`.
2. Document the security trade-off of `allow_encoded_slash: true` for APIs like GitLab where `%2F` in a path segment is normal (ID-or-path parameters). Is it safe if the endpoint has no path-scoped rules? A provider-profile example for GitLab would help.

Versions: openshell 0.1.4 (deb), podman driver, glab 1.128.0, Linux x86_64.
