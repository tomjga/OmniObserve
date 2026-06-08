# Scenario B — EC2 + AKS

**1 AWS EC2 (plain IaaS) + Azure AKS (managed Kubernetes)**. The point is the *asymmetry*: one
side you operate by hand, the other side a scheduler self-heals and scales for you.

```
            ┌──────────── DNS (health-checked, active-passive) ───────────┐
            └──────────────┬──────────────────────────┬───────────────────┘
                  primary   │                          │  standby/scale
            ┌───────────────▼────────┐      ┌──────────▼──────────────────┐
            │ AWS · EC2 t4g.small    │      │ Azure · AKS (Free tier CP)  │
            │ app + agent (systemd)  │      │ Deployment + agent DaemonSet│
            └───────────┬────────────┘      └──────────┬──────────────────┘
                        └──────────── OTLP ────────────┘ → OmniObserve → LGTM
```

## Reliability
- **HA mechanism:** AKS self-heals (reschedules crashed pods, supports rolling updates, HPA
  autoscaling) — real intra-cluster resilience the single EC2 box cannot match. DNS still
  provides the cross-cloud failover between the two sides.
- **Asymmetry to highlight:** the EC2 side has *no* self-healing; if its app crashes, only a host
  health check notices. This is exactly the gap managed K8s closes.

## Disaster Recovery (enable-dr.sh)
- **Strategy:** active-passive — EC2 primary, AKS as the scalable standby (or flip it).
- **RTO:** ~1–5 min cross-cloud (DNS) ; **seconds** intra-AKS (pod reschedule). **RPO:** as Scenario A.
- **DR drill:** scale AKS to 0 / cordon nodes and confirm failover; or kill the EC2 primary.

## Pricing (approximate — 2026-06)
| Item | Type | ~$/mo |
|------|------|-------|
| AWS EC2 | t4g.small | $5–12 |
| AKS nodes | 2× Standard_B2s | $80–110 |
| AKS control plane | **Free tier = $0** | $0 |
| LB / DNS / egress | | $5–15 |
| **Total** | | **~$90–140** |

> Pricing lesson: AKS's control plane is **free** on the default tier — you pay only for nodes.
> Contrast with Scenario C, where EKS bills ~$73/mo for the control plane alone.

## Pros
- Real self-healing + autoscaling on the K8s side; rolling deploys; declarative ops.
- Still cloud-diverse (AWS + Azure) for whole-provider outages.
- Free AKS control plane keeps the managed-K8s premium modest.

## Cons
- Two operating models to run (a VM *and* a cluster) — more cognitive load.
- Node fees dominate cost; idle clusters are wasteful for a tiny app.
- Cross-cloud data consistency still unsolved by the platform.

## When to choose
You want managed-K8s benefits (self-healing, autoscaling, GitOps) but only on one cloud, with a
cheap second-cloud foothold for DR — and you want AKS's free control plane to keep the bill down.
