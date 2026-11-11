# [Feature] Allow one provider credential in more than one declared header

Repo: NVIDIA/OpenShell — no existing issue found (searched "credential_placement_mismatch", "placement multiple headers", 2026-11-11).

## Problem
Since v0.1.8 a static credential is substituted only in its declared placement (`auth_style` + `header_name`). Some services accept the same token in several headers, and different clients pick different ones. GitLab: git/curl/MCP use `Authorization`, glab uses `PRIVATE-TOKEN` for PATs. One credential can declare only one placement, so one client always gets `403 credential_placement_mismatch`.

## Repro (v0.1.8, podman driver)
1. Profile credential `auth_style: basic`, `header_name: authorization`, endpoint gitlab.com:443.
2. In sandbox: `glab mr list` → 403 `credential_placement_mismatch`; `git ls-remote` → ok.
3. Change to `auth_style: header`, `header_name: private-token`: glab ok, git and `curl -H "Authorization: Bearer $GITLAB_TOKEN"` → 403.

## Expected
A credential can list several allowed placements, e.g. `placements: [{header: authorization}, {header: private-token}]`, each still bound to the profile endpoints.

## Versions
openshell 0.1.8 (gateway + supervisor), glab 1.128.0, Ubuntu 26.04, podman 6.2.0.
