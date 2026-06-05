---
id: INC-2026-0003
title: Tempo rejected every trace write — "at least 2 live replicas required"
date: 2026-06-04
severity: SEV3
status: resolved
services: [tempo, otel-collector]
detection: Collector otlp/tempo exporter logging repeated export failures; Tempo search returning "too many unhealthy instances in the ring"
slo_impact: none (caught in pre-prod validation) — but would have meant 100% trace loss
tags: [tempo, tempo-distributed, replication-factor, quorum, ingester, helm, single-node]
remediation:
  - Set ingester.config.replication_factor to 1 when running a single ingester
---

## Summary
After migrating from the deprecated single-binary `grafana/tempo` chart to
`tempo-distributed` (microservices mode), **no traces landed**. The OTel Collector's
`otlp/tempo` exporter logged `rpc error: code = Unavailable desc = at least 2 live
replicas required, could only find 1` on every batch; Tempo's search API returned
`too many unhealthy instances in the ring`.

## Timeline
- Tempo install succeeded; all 6 components reported `1/1 Running`.
- Collector received spans (debug exporter showed them) but every forward to the
  distributor failed; Tempo held zero traces.

## Root cause
`tempo-distributed` defaults to `ingester.config.replication_factor: 3`. The distributor
requires a write quorum (2 of 3) before acknowledging a write. Tuned for a single-node
local cluster, the chart ran **one** ingester — so quorum was mathematically impossible
and every write was refused. The components were individually healthy, which masked the
problem: this is a *topology/config* failure, not a crash.

## Resolution
Set `replication_factor: 1` to match the single ingester:

```yaml
ingester:
  replicas: 1
  config:
    replication_factor: 1
```

Rolled the ingester/distributor; writes succeeded and traces became queryable.

## Lessons / prevention
- When scaling a quorum-based system down to one replica, **the replication factor must
  scale with it** — "1 replica" and "RF=3" are silently incompatible.
- "All pods `Running` and `Ready`" does not mean the data path works. Validate with an
  actual **write + read**, not just pod health.
- A future RCA tagged `replication-factor` should check `replicas` vs the configured RF
  first.
