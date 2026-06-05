---
id: INC-2026-0006
title: Alerts fired but never reached the webhook — injected namespace matcher
date: 2026-06-04
occurred: 2026-06-04T21:30:00-06:00
severity: SEV3
status: resolved
services: [alertmanager, remediator]
detection: Alert active in Alertmanager for 6+ minutes, but the remediator webhook was never called (remediator_alerts_received_total stayed 0)
slo_impact: none (caught wiring the control loop in pre-prod) — but in prod this is a silent auto-remediation/notification outage
tags: [alertmanager, prometheus-operator, alertmanagerconfig, routing, matchers, multi-namespace, silent-failure]
remediation:
  - Set the Alertmanager's alertmanagerConfigMatcherStrategy.type to None so routes match on explicit labels, not an injected namespace matcher
---

## Summary
Wiring the remediator control loop: an SLO alert (`ApiServiceHighErrorRate`) fired and
showed **active** in Alertmanager, but the remediator's `/webhook` was **never called**.
The alert was silently delivered to the catch-all `null` receiver instead of the
remediator route.

## Timeline
- Drove errors at api-service; the alert went active in Alertmanager within ~75s.
- For 6+ minutes `remediator_alerts_received_total` stayed `0`.
- Inspecting the alert: `receivers: ["null"]` — it never matched the remediator route.

## Root cause
The Prometheus Operator's default **`alertmanagerConfigMatcherStrategy: OnNamespace`**
injects a `namespace="<AlertmanagerConfig's namespace>"` matcher into **every** route it
generates from an `AlertmanagerConfig`. The remediator route therefore required both
`omniobserve_remediate="true"` **and** `namespace="monitoring"`.

But the alert's expression aggregates away all series labels
(`sum(...)/clamp_min(sum(...))`), so the alert had **no `namespace` label** — it carried
only the labels set in the rule. The `namespace="monitoring"` matcher could never match,
so the alert fell through to the default `null` receiver. The failure was **silent**:
the alert looked perfectly healthy in Alertmanager; only the *absence* of a downstream
call revealed it.

This is also an **architectural** mismatch: `OnNamespace` is built for multi-tenant
isolation (a team's config only routes its own namespace's alerts). The remediator is a
**cross-cutting** control plane that must receive opt-in alerts from *any* namespace
(api-service in `monitoring`, the demo in `otel-demo`). A namespace matcher defeats that
by design.

## Resolution
Set `alertmanagerConfigMatcherStrategy.type: None` on the Alertmanager (via the
kube-prometheus-stack value in `bootstrap.sh`). Routing then keys on our explicit
`omniobserve_remediate="true"` label, regardless of namespace. The remediator received
the alert within 15s.

## Lessons / prevention
- An alert being **active in Alertmanager is not proof it was routed**. Verify the
  receiver (`/api/v2/alerts` → `receivers`), not just the alert state.
- Routing on a label requires every alert to **carry** that label — aggregated alert
  expressions drop series labels like `namespace`, so don't rely on them implicitly.
- For a **cross-namespace** consumer (a control loop, a global notifier), the operator's
  default `OnNamespace` matcher strategy is the wrong default — route on an explicit
  opt-in label and disable the injected namespace matcher.
- A future RCA tagged `routing` + `silent-failure` should check **receiver resolution**
  and **label presence vs matchers** before anything else.
