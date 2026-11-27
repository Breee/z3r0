---
title: Documentation
weight: 1
cascade:
  - target:
      path: "/docs/*"
    sidebar:
      open: true
description: User documentation for zero, the local version.
---

zero runs a coding agent in a sandbox on your machine. The agent gets a copy of your repositories, a short-lived GitLab token for exactly these projects and network access to the hosts your policy lists. Work leaves the sandbox in one way: `git push` to the branch `zero/<workspace>`. You review that branch and open the merge request.

## Pages

{{< cards >}}
  {{< card link="getting-started" title="Getting started" subtitle="Install zero and start the first workspace." >}}
  {{< card link="commands" title="Commands" subtitle="Every command with an example." >}}
  {{< card link="policy" title="Policy" subtitle="Hosts, capabilities and tools of a workspace." >}}
  {{< card link="providers" title="Model providers" subtitle="Where the model login lives, without and with the model proxy." >}}
  {{< card link="security" title="Security" subtitle="What the agent can do, what it cannot do, what is open." >}}
{{< /cards >}}

## Reference

These documents are written for people who work on zero itself. They live in the repository:

- [Architecture](https://github.com/Breee/z3r0/blob/main/docs/architecture)
- [Decisions (ADR)](https://github.com/Breee/z3r0/blob/main/docs/adr/README.md)
- [Attack paths](https://github.com/Breee/z3r0/blob/main/docs/attack-paths.md)
- [Workspace specification](https://github.com/Breee/z3r0/blob/main/docs/spec/workspaces.md)
- [Roadmap](https://github.com/Breee/z3r0/blob/main/docs/roadmap.md)

## Limits of this version

- Linux on x86_64 with systemd and rootless podman.
- GitLab only (gitlab.com or a self-hosted instance).
- The model comes from the GitHub Copilot login of opencode on your host.
