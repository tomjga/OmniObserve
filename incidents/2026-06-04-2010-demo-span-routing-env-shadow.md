---
id: INC-2026-0004
title: OTel Demo spans silently dropped — env override shadowed by component-level value
date: 2026-06-04
occurred: 2026-06-04T20:10:00-06:00
severity: SEV3
status: resolved
services: [opentelemetry-demo, otel-collector]
detection: Collector debug exporter showed only api-service spans; Tempo never listed demo services despite 200-OK browse traffic
slo_impact: none (caught in pre-prod validation) — silent 100% trace loss for the demo workload
tags: [opentelemetry, helm, env-vars, kubernetes, config-layering, silent-failure]
remediation:
  - Override OTEL_COLLECTOR_NAME (which the per-service endpoint interpolates) instead of OTEL_EXPORTER_OTLP_ENDPOINT
---

## Summary
With the demo's bundled collector disabled, every service was supposed to export to
OmniObserve's collector via a chart-wide `default.envOverrides` setting
`OTEL_EXPORTER_OTLP_ENDPOINT`. Browse traffic returned `200`, but **no demo spans ever
reached the collector or Tempo** — only `api-service` (a separate workload) appeared.

## Timeline
- Generated browse traffic (`/api/products`, `/api/recommendations`) — all `200`.
- Collector debug exporter showed `service.name=api-service` only.
- Inspecting a demo pod's env revealed `OTEL_EXPORTER_OTLP_ENDPOINT` defined **twice**.

## Root cause
The env list on each demo pod contained:
```
OTEL_EXPORTER_OTLP_ENDPOINT=http://otelcol.monitoring.svc.cluster.local:4317   # ours (default-level)
OTEL_EXPORTER_OTLP_ENDPOINT=http://$(OTEL_COLLECTOR_NAME):4317                  # demo's (component-level, LAST)
```
Each service re-declares the endpoint at the **component** level, which is appended
*after* the chart-wide default override. In Kubernetes, when an env name repeats, the
**last definition wins** — so the demo's value (`$(OTEL_COLLECTOR_NAME)` →
`otel-collector`, a Service we had disabled) won, and spans were sent to a name that
didn't resolve. No error surfaced because OTLP export failures are async and the app
doesn't block on them.

## Resolution
Override **`OTEL_COLLECTOR_NAME`** instead — the variable the per-service endpoint
interpolates from — so the (last-winning) component endpoint resolves to our collector:
```yaml
default:
  envOverrides:
    - name: OTEL_COLLECTOR_NAME
      value: otelcol.monitoring.svc.cluster.local
```
Verified `frontend`, `product-catalog`, `cart`, `recommendation` spans then reached the
collector and Tempo.

## Lessons / prevention
- *Where* config is layered matters as much as its value. A "default" override loses to a
  component-level redefinition of the same key.
- Telemetry misconfiguration is **silent** — the app keeps serving `200`s while traces go
  nowhere. Always confirm spans at the destination, not just that requests succeed.
- Prefer overriding the **indirection point** (`OTEL_COLLECTOR_NAME`) the templates were
  designed around, rather than fighting the value they compute.
