## 25. Initial rollout

### Phase 0 — OpenShell feasibility spike

Prove the load-bearing assumptions with a corrupted-agent test harness:

1. OpenShell sandbox cannot read developer host secrets or unrelated repositories; [Z-P0-1]
2. arbitrary agent child processes inherit the intended containment; [Z-P0-2]
3. unknown egress destinations are denied; [Z-P0-3]
4. OpenShell provider placeholder credentials cannot be extracted as reusable real credentials; [Z-P0-4]
5. OpenBao-backed GitLab credentials can be substituted behind OpenShell without entering the sandbox; [Z-P0-5]
6. feature-branch push works while protected-branch push remains denied by Z3R0 + GitLab controls; [Z-P0-6]
7. `glab pipeline status` works with narrow GitLab authority; [Z-P0-7]
8. container build/run works without exposing the normal Docker socket; [Z-P0-8]
9. hostile tests/Dockerfiles cannot reach host secrets or arbitrary network destinations; [Z-P0-9]
10. Z3R0 middleware sanitizes representative Kubernetes specs/logs before they reach the sandbox; [Z-P0-10]
11. OCSF/OTel/Z3R0 traces correlate epoch → process/tool → policy → credential → target; [Z-P0-11]
12. hard revoke/quarantine cuts remote authority within the stated budget. [Z-P0-12]

### Phase 1 — Go developer laptop preview

Use **OpenShell + rootless Podman** as the default runtime and support:

```text
code
→ go test
→ local container build/run
→ signed commit
→ push feature branch
→ glab pipeline status
→ fix
→ create MR
```

Integrate OpenCode first; add VS Code where the custom endpoint/workspace attachment model works cleanly.

### Phase 2 — Kubernetes operator preview

Support:

```text
DEV/QA read/write according to policy
+
PROD one-shot sanitized diagnostics
→ diagnose
→ GitOps repository change
→ DEV/QA validation
→ MR / CI / human-controlled promotion
```

No production mutation capability exists for normal agents.

### Phase 3 — stronger isolation

Enable OpenShell MicroVM profiles for untrusted repositories, third-party code, hostile build workloads or organization-defined higher-risk tasks.

### Phase 4 — remote/shared execution

Adopt OpenShell Kubernetes compute for centrally managed agent execution when there is a concrete team/fleet requirement.

---

