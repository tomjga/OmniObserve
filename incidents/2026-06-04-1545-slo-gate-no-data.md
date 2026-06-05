---
id: INC-2026-0001
title: Healthy canary auto-aborted by SLO gate (no-data treated as failure)
date: 2026-06-04
occurred: 2026-06-04T15:45:00-06:00
severity: SEV4
status: resolved
services: [api-service]
detection: Argo Rollouts canary auto-aborted during a clean (error-free) deploy
slo_impact: none (caught in pre-prod validation)
tags: [slo, argo-rollouts, prometheus, promql, false-positive, gating]
remediation:
  - Wrap the analysis query in "( ... ) or vector(0)" so empty/no-data evaluates to 0
---

## Summary
A canary deploy with **zero errors** was automatically aborted and rolled back. The
SLO AnalysisTemplate's 5xx-ratio query returned an *empty* result rather than `0`, and
Argo Rollouts scored the empty measurement as a failure.

## Timeline
- Re-triggered a clean rollout (no fault injection) to demonstrate the happy path.
- The background AnalysisRun reported no/empty data instead of `0`.
- After `failureLimit` checks, the rollout aborted and scaled the canary back down.

## Root cause
`sum(rate(http_requests_total{code=~"5.."}[1m]))` matches **no series** when there are
no 5xx responses, so PromQL returns an empty vector — not `0`. The division produced an
empty result, which the analysis provider treated as a failed measurement.

## Resolution
Wrapped the expression in `( ... ) or vector(0)` so empty / no-traffic windows evaluate
to `0`, letting a clean canary promote while a real 5xx spike still fails the gate.

## Lessons / prevention
- In SLO gating, **empty ≠ zero** — always make "no data" an explicit, intended value.
- Validate the **happy path** of a gate, not only its failure path.
- A future RCA tagged `promql` + `false-positive` should check for unmatched-series /
  empty-vector handling first.
