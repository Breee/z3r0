## 18. Observability and evidence

Every session gets a trace/session ID.

Example:

```text
session = dev-session-4711

1. developer prompt
2. agent reads issue #142
3. agent reads repository
4. agent modifies files
5. go test ./...
6. container.build
7. container.run integration test
8. git commit
9. feature branch push
10. credential lease issued
11. GitLab CI starts
12. glab pipeline status
13. pipeline failure inspected
14. agent fixes code
15. push
16. pipeline passes
17. MR created
18. merge approval requested
```

Evidence should connect:

```text
human
→ agent/session
→ model call
→ tool/CLI operation
→ capability decision
→ approval
→ credential use
→ target API
→ result
```

Minimum useful telemetry:

- caller/session identity;
- requested capability;
- target resource;
- policy decision;
- reason for denial;
- approval identity and timestamp;
- credential lease ID/TTL without exposing secret value;
- process/tool invocation;
- destination host;
- GitLab/Kubernetes request metadata;
- result/status;
- cost/token usage for AI calls.

---

