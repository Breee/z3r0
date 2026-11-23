---
# The site root redirects to /docs/ (see layouts/index.html); only the llms.txt
# output of this page is rendered.
title: zero
description: Sandboxes for coding agents. The agent works on your repositories and cannot do damage outside its branch.
llmsDescription: |
  zero runs a coding agent (opencode) in an OpenShell sandbox on a Linux host. The agent gets a copy of the
  repositories of a workspace and a short-lived GitLab token for these projects. Its only way out is git push
  to the branch zero/<workspace> through a push gate. Policy: .zero/policy.yaml, read on the host.
---
