# OTel Collector

Vendor-neutral fan-out layer. `api-service` exports OTLP/gRPC to the collector
(`:4317`), which routes each signal to the LGTM stack:

| Signal  | Exporter                | Backend            |
|---------|-------------------------|--------------------|
| traces  | `otlp/tempo`            | Tempo              |
| metrics | `prometheusremotewrite` | Prometheus / Mimir |
| logs    | `otlphttp/loki`         | Loki (OTLP ingest) |

The `debug` exporter is wired into every pipeline so telemetry is visible in the
collector logs even before the backends are reachable.

## Local dev

```bash
docker compose -f collector/docker-compose.yaml up      # start the collector
# in another shell:
cd application && go run .                               # api-service -> :4317
curl localhost:8080/benchmark?delay=20                  # generate a span
```

The backend endpoints (`tempo`, `prometheus-server`, `loki-gateway`) are in-cluster
Service names; in a backend-less local run only the `debug` exporter succeeds, which
is enough to confirm instrumentation. Override the target from the service side with
`OTEL_EXPORTER_OTLP_ENDPOINT` (defaults to `localhost:4317`).
