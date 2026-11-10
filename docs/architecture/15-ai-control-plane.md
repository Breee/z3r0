## 15. AI control plane

LiteLLM is the default single AI control plane for the developer path.

Responsibilities:

- model/provider abstraction;
- provider keys kept off developer machines;
- model allow-lists;
- budgets/rate limits;
- MCP routing;
- A2A routing;
- tool visibility/permission;
- tool trust classification/guardrails;
- downstream signed identity where supported;
- model/agent telemetry.

Important principle:

> Tools the identity cannot use should ideally not be visible to the agent at all.

However:

> **Visibility is not authorization.**

Every sensitive action must still be enforced at the actual runtime/credential/target boundary.

`agentgateway` remains optional for cluster-internal east-west traffic where mTLS, Gateway API integration, CEL-style policy, or inference routing justify a separate network-layer gateway.

---

