# Kubernetes access proof (#123)

Design and results: `docs/scenarios/kubernetes-access.md` sections 9 and 10.

| File | What |
|---|---|
| `capabilities.yaml` | one Role per zero capability, per namespace |
| `session.yaml` | what a human needs to start a session, per namespace |
| `guard.yaml` | admission policies, once per cluster (needs Kubernetes ≥ 1.30) |
| `demo.yaml` | demo workload with a Secret, a ConfigMap and an admin ServiceAccount |
| `session.sh` | start/stop a session with the human's kubeconfig |
| `probe.sh` | the agent's calls with the session token |
