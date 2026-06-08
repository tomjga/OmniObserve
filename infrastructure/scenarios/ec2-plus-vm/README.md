# Scenario A — EC2 + Azure VM

**1 AWS EC2 + 1 Azure Linux VM**, same app on both, one observability agent each, fronted by
health-checked DNS. The cheapest way to demonstrate genuine cross-cloud high availability.

```
                    ┌────────────── DNS (health-checked) ──────────────┐
                    │            app.example.com  (failover)            │
                    └───────────────┬───────────────────┬──────────────┘
                          active    │                   │    active
                   ┌────────────────▼──────┐   ┌─────────▼─────────────┐
                   │  AWS · EC2 t4g.small   │   │ Azure · VM B2s        │
                   │  app + Alloy/OTel agent│   │ app + Alloy/OTel agent│
                   └───────────┬────────────┘   └──────────┬───────────┘
                               └─────── OTLP ──────────────┘
                                  OmniObserve collector → LGTM
```

## Reliability
- **HA mechanism:** two independent failure domains (different clouds, different regions). DNS
  health checks withdraw an unhealthy endpoint; the survivor keeps serving.
- **Single points of failure to call out:** the DNS provider itself, and shared app state/data
  (stateless demo app avoids this; a real app needs cross-cloud data replication — that's DR).

## Disaster Recovery (enable-dr.sh)
- **Strategy:** active-active across clouds → loss of an entire cloud ≈ a routing change.
- **RTO:** ~1–5 min (DNS health-check interval + TTL).  **RPO:** ~0 for stateless; = replication
  lag once you add a shared datastore.
- **DR drill:** `destroy.sh --scenario ec2+vm --cloud aws` then confirm traffic on Azure only.

## Pricing (approximate — 2026-06, on-demand, us-east-1 / eastus)
| Item | Type | ~$/mo |
|------|------|-------|
| AWS EC2 | t4g.small (spot ~$5) | $5–12 |
| Azure VM | Standard_B2s | $8–15 |
| DNS health checks / egress | Route 53 / TM | $1–3 |
| **Total** | | **~$15–30** |

## Pros
- Cheapest cross-cloud HA; no managed-K8s control-plane fees.
- Simplest mental model — two boxes, one DNS name.
- Truly cloud-diverse: survives a whole-provider outage.

## Cons
- You own orchestration: deploys, scaling, patching are manual (no scheduler).
- No self-healing of a single box (no K8s to reschedule a crashed pod) — DNS only routes around a *dead host*.
- Data consistency across clouds is on you.

## When to choose
A small, mostly-stateless service that must survive a full-cloud outage on a tight budget; or
as the teaching baseline before reaching for managed Kubernetes (Scenarios B/C).
