# Phase 1.5 — Real telemetry (a system worth observing)

**The problem.** Until now the telemetry came from a synthetic KPI simulator. An
observability platform is only convincing when it observes a *real*, distributed system —
and an RCA copilot (Phase 2) is only useful if it has real traces and logs to reason over.

**What this phase builds.**
- The **OpenTelemetry Demo** — a real, polyglot, OTel-native microservices app — deployed
  as a workload and routed into *our* collector (its bundled observability disabled).
- **Tempo** for distributed traces; the collector fans traces → Tempo, metrics →
  Prometheus, logs → Loki.
- A **Grafana Tempo datasource**, so traces are explorable next to the existing metrics.

**Why it matters.**
- **Real distributed traces** — a single request crossing many services is exactly what
  dashboards and an LLM RCA need; synthetic `rand()` can't produce it.
- **Built-in fault injection** (flagd feature flags) — realistic regressions on demand,
  so the auto-remediation story is real, not faked.
- This is the data Phase 2's copilot reasons over, and the corpus it learns from.

**See it:**

```bash
./bootstrap.sh            # core stack (if not already up)
./bootstrap-telemetry.sh  # Tempo + collector + Grafana datasource + the demo
```

Then Grafana → Explore → **Tempo** → search recent traces. See
[`workloads/otel-demo/`](../workloads/otel-demo/) for details and fault injection.
