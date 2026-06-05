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

**On the LLM service.** The demo ships a heavy local `llm` model service. We disable it
and treat LLM work as a **delegated, vendor-agnostic** call instead — an
OpenAI-compatible endpoint chosen by config (`LLM_BASE_URL`/`LLM_MODEL`/`LLM_API_KEY`),
never a provider baked into code. This is the same boundary Phase 2's RCA copilot is
built on: swap Gemini, Claude, OpenAI, or a local model by changing env only.

**Real bugs we caught standing this up** (now in the [incident corpus](../incidents/)):
- *Tempo dropped every trace.* `tempo-distributed` defaults to
  `replication_factor: 3`; with one ingester the distributor refused all writes
  (`at least 2 live replicas required`). A single-node deploy must set RF to 1 — the
  kind of dependency that looks fine in `helm install` and only fails at write time.
- *Demo spans went to a black hole.* Each service re-declares
  `OTEL_EXPORTER_OTLP_ENDPOINT=http://$(OTEL_COLLECTOR_NAME):4317` at the component
  level, which shadows a default-level endpoint override (last env wins). The fix is to
  override `OTEL_COLLECTOR_NAME`, not the endpoint — a reminder that *where* config is
  layered matters as much as the value.
- *The node ate itself.* The bundled load generator on an under-provisioned VM pegged
  CPU until the kube-apiserver couldn't complete TLS handshakes. Right-sizing the
  workload to the cluster (drop the load generator, stage startup) is the fix.

**See it:**

```bash
./bootstrap.sh            # core stack (if not already up)
./bootstrap-telemetry.sh  # Tempo (tempo-distributed) + collector + datasource + demo
```

Then Grafana → Explore → **Tempo** → search recent traces. See
[`workloads/otel-demo/`](../workloads/otel-demo/) for details and fault injection.
