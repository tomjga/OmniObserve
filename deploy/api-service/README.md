# api-service Helm chart

Deploys the OTel-instrumented `api-service` to Kubernetes. Replaces the earlier
incomplete `application/deploy.yaml` (a template with no `Chart.yaml`).

```bash
helm upgrade --install api-service deploy/api-service -n monitoring
```

What it creates:

- **Deployment** — non-root, read-only rootfs, dropped capabilities, liveness/readiness
  on `/healthz`, and `OTEL_EXPORTER_OTLP_ENDPOINT` pointing at the collector.
- **Service** — ClusterIP on port 8080 (`http`).
- **ServiceMonitor** (Prometheus Operator) — scrapes `/metrics`; `jobLabel` makes
  Prometheus emit `job="api-service"`, which the [SLO rules](../../slo/) select on.

Key values (`values.yaml`): `image.repository/tag`, `otel.endpoint`,
`serviceMonitor.enabled`, `resources`. The image is built and signed by CI
(see `.github/workflows/`).

> In Phase 1 this Deployment is converted to an Argo Rollout for canary delivery.
