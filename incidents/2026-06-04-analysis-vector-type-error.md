---
id: INC-2026-0002
title: Canary analysis errors with "mismatched types []float64 and float64"
date: 2026-06-04
severity: SEV4
status: resolved
services: [api-service]
detection: Argo Rollouts AnalysisRun Phase=Error, canary aborted
slo_impact: none (caught in pre-prod validation)
tags: [argo-rollouts, prometheus, promql, analysis, type-error, gating, flaky]
remediation:
  - Wrap the analysis query in scalar() so Prometheus returns a scalar, not a vector
---

## Summary
After fixing the no-data case ([[INC-2026-0001]]), the canary analysis still failed —
**intermittently**. The AnalysisRun reported `Phase: Error`, message
`invalid operation: < (mismatched types []float64 and float64)`.

## Timeline
- A clean rollout (`good3`) promoted successfully right after the no-data fix.
- A later clean rollout (rev 4) aborted with the type-mismatch error.

## Root cause
The query returned a Prometheus **instant vector**. Argo Rollouts reads a vector
result as `[]float64`, so `successCondition: result < 0.05` compared a slice to a
scalar and errored. It was intermittent because Argo special-cases a *single*-element
vector as `float64` — so when the vector had exactly one sample it worked, otherwise it
errored.

## Resolution
Wrapped the expression in `scalar( ... )`, which makes Prometheus return a **scalar
type**. Argo always reads a scalar as `float64`, so the comparison is stable regardless
of element count. `or vector(0)` stays inside to keep the no-5xx case at `0` (not NaN).

## Lessons / prevention
- Argo Rollouts Prometheus `successCondition` queries should return a **scalar** — wrap
  vector expressions in `scalar()`.
- Intermittent gate failures are a smell for **result-type/cardinality** issues, not load.
- A future RCA tagged `argo-rollouts` + `promql` should check the result *type* first.
