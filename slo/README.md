# SLOs (as code)

SLOs are declared once in [`api-service.slo.yaml`](api-service.slo.yaml) and compiled
to Prometheus rules with [Sloth](https://sloth.dev). The generated
[`api-service.rules.yaml`](api-service.rules.yaml) is committed because Prometheus
loads it directly — and CI fails if it drifts from the spec.

| SLO            | Objective | SLI source metric                     |
|----------------|-----------|---------------------------------------|
| availability   | 99.5%     | `http_requests_total` (non-5xx ratio) |
| latency-p95    | 95.0%     | `http_request_duration_seconds`       |

## Regenerate

```bash
sloth generate -i slo/api-service.slo.yaml -o slo/api-service.rules.yaml
```

The rules expose `slo:sli_error:ratio_rateXX` recordings and multi-window
multi-burn-rate alerts (`ApiServiceHighErrorRate`, `ApiServiceHighLatency`). Phase 1's
Argo Rollouts `AnalysisTemplate` queries these recordings to gate canary promotion.

> The SLI queries select `job="api-service"`; the Prometheus scrape job (see the Helm
> chart) must use that job name.
