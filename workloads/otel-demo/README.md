# Real workload: OpenTelemetry Demo

Adds a **real, distributed, OTel-native** application as a system under observation,
so the LGTM stack (and Phase 2's RCA copilot) work on real traces/metrics/logs
instead of synthetic data. The demo also ships **built-in fault injection** (flagd
feature flags), which makes the auto-remediation story real rather than simulated.

> Do this **after** the core demo (`bootstrap.sh`) is validated. It is resource-heavy
> (~20 pods) — give Rancher Desktop ≥ 6 GB / 4 CPU.

## 1. Telemetry backends (traces, logs, collector)

The core bootstrap only installs Prometheus. Add Tempo, Loki, and the collector so
the demo's traces/logs land somewhere:

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

# Traces
helm upgrade --install tempo grafana/tempo -n monitoring

# Logs (single-binary is fine for local)
helm upgrade --install loki grafana/loki -n monitoring \
  --set deploymentMode=SingleBinary --set loki.commonConfig.replication_factor=1 \
  --set loki.storage.type=filesystem --set singleBinary.replicas=1 \
  --set 'loki.auth_enabled=false'

# Collector — fullnameOverride 'otelcol' so the Service matches our configs.
# Supply OmniObserve's config (collector/otelcol-config.yaml) as the chart's config:
helm upgrade --install otelcol open-telemetry/opentelemetry-collector -n monitoring \
  --set mode=deployment --set fullnameOverride=otelcol \
  --set image.repository=otel/opentelemetry-collector-contrib \
  -f <(yq '{"config": load("../../collector/otelcol-config.yaml")}' 2>/dev/null \
       || python3 -c "import yaml,sys;print(yaml.safe_dump({'config':yaml.safe_load(open('../../collector/otelcol-config.yaml'))}))")
```

If the last command's inline config trick doesn't fit your tooling, copy
`collector/otelcol-config.yaml` under a `config:` key into a values file and pass it
with `-f`.

## 2. The demo app

```bash
helm upgrade --install otel-demo open-telemetry/opentelemetry-demo \
  -n otel-demo --create-namespace -f values.yaml
```

`values.yaml` disables the demo's bundled observability and points every service's
`OTEL_EXPORTER_OTLP_ENDPOINT` at our collector.

## 3. Verify

- Grafana → Explore → **Tempo**: traces spanning frontend → cartservice → … should appear.
- Grafana → Explore → **Loki**: structured logs from the demo services.
- The demo's **flagd** feature flags inject faults (e.g. `cartServiceFailure`,
  `paymentServiceUnreachable`) — flip one and watch the error signal propagate. This is
  the realistic regression source for the auto-remediation work in Phase 2.

> Next: once this is solid, the same pattern (instrument → export OTLP to `otelcol`)
> is how the personal **Dashboard** project plugs in as the flagship owned workload.
