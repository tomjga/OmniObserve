# Real workload: OpenTelemetry Demo

Adds a **real, distributed, OTel-native** application as a system under observation, so
the LGTM stack (and Phase 2's RCA copilot) work on real traces/metrics/logs instead of
synthetic data. The demo also ships **built-in fault injection** (flagd feature flags),
which makes the auto-remediation story real rather than simulated.

> Run **after** `bootstrap.sh`. It's resource-heavy (~15–20 pods) — give Rancher Desktop
> ≥ 6 GB / 4 CPU.

## Deploy

```bash
./bootstrap-telemetry.sh
```

That installs **Tempo** in microservices mode
([`tempo-distributed`](tempo-distributed-values.yaml), from the maintained
`grafana-community` repo — the single-binary `grafana/tempo` chart is deprecated), the
**OTel Collector** (using
[`collector/otelcol-config.yaml`](../../collector/otelcol-config.yaml) via a ConfigMap),
a **Grafana Tempo datasource** (query-frontend on `:3200`), and the **OpenTelemetry
Demo** wired to our collector.

[`values.yaml`](values.yaml) disables the demo's bundled observability and points every
service's `OTEL_EXPORTER_OTLP_ENDPOINT` at `otelcol.monitoring.svc.cluster.local:4317`.

> Verify the chart's value keys against your version first — subchart toggle names shift
> between releases: `helm show values open-telemetry/opentelemetry-demo | less`.

## Verify

- **Grafana → Explore → Tempo**: traces spanning `frontend → cartservice → …`.
- **Prometheus**: the demo's metrics arrive via the collector's remote-write.
- **flagd** feature flags inject faults (e.g. `cartServiceFailure`,
  `paymentServiceUnreachable`) — the realistic regression source for Phase 2.

> Logs (Loki) are left out of the first pass to keep it robust; the collector's logs
> pipeline will log export errors until Loki is added — traces + metrics flow regardless.

> Next: the same pattern (instrument → export OTLP to `otelcol`) is how the personal
> **Dashboard** project plugs in as the flagship owned workload.
