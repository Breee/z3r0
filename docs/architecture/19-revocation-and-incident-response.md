## 19. Revocation and incident response

The stack must support remote response while the workstation or agent is compromised.

Actions should include:

- disable agent/session identity;
- revoke OpenBao leases;
- disable LiteLLM key/team/session;
- block a tool/capability;
- destroy sandbox/VM;
- revoke device registration;
- stop cluster agent workload;
- terminate A2A task.

The security objective is a measurable **time-to-effect** budget defined by the maximum relevant TTL/cache duration.

Revocation must be tested operationally, not merely documented.

---

