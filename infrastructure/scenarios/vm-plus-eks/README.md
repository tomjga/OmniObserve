# Scenario C — Azure VM + EKS

**1 Azure Linux VM (plain IaaS) + AWS EKS (managed Kubernetes)** — the mirror image of Scenario B,
clouds swapped, so the **EKS-vs-AKS control-plane cost** difference is unmistakable.

```
            ┌──────────── DNS (health-checked, active-passive) ───────────┐
            └──────────────┬──────────────────────────┬───────────────────┘
                  primary   │                          │  standby
            ┌───────────────▼────────┐      ┌──────────▼──────────────────┐
            │ AWS · EKS (CP ~$73/mo) │      │ Azure · VM B2s              │
            │ Deployment + DaemonSet │      │ app + agent (systemd)       │
            └───────────┬────────────┘      └──────────┬──────────────────┘
                        └──────────── OTLP ────────────┘ → OmniObserve → LGTM
```

## Reliability
- **HA mechanism:** EKS self-heals + autoscales (same K8s benefits as Scenario B's AKS); DNS
  handles cross-cloud failover to the Azure VM.
- **The lesson:** functionally identical to Scenario B, but EKS bills a flat **~$0.10/h (~$73/mo)
  per cluster** for the control plane — money AKS's Free tier doesn't charge. Same architecture,
  different invoice. This is the kind of trade-off a Systems Architect is paid to know.

## Disaster Recovery (enable-dr.sh)
- **Strategy:** active-passive — EKS primary, Azure VM as the cheap standby.
- **RTO/RPO:** as Scenario B (seconds intra-cluster; 1–5 min cross-cloud).
- **DR drill:** `destroy.sh --scenario vm+eks --cloud aws` → confirm the Azure VM serves alone.

## Pricing (approximate — 2026-06)
| Item | Type | ~$/mo |
|------|------|-------|
| EKS control plane | flat per-cluster | **~$73** |
| EKS nodes | 2× t4g.small (spot) | $10–25 |
| Azure VM | Standard_B2s | $8–15 |
| LB / DNS / egress | | $5–15 |
| **Total** | | **~$95–140** |

### Near-$0 variant — k3s on EC2
Swap the `aws-eks` module for **k3s on a single t4g.small EC2** (`modules/aws-ec2` + a k3s
install in user_data). You get a real Kubernetes API and DaemonSet-based observability for ~$5–12/mo
— you self-manage the control plane instead of paying AWS for it. Great for demos; you trade the
managed-HA control plane for cost.

## Pros
- Full managed-K8s on AWS (the most common enterprise target) + a cheap cross-cloud standby.
- Cleanly exposes the EKS control-plane premium vs AKS — a strong architecture talking point.

## Cons
- Most expensive of the three by default (EKS control-plane fee).
- Same two-operating-models overhead as Scenario B.

## When to choose
Your primary is AWS-on-EKS (org standard / ecosystem) and you want a low-cost Azure DR foothold —
or you're explicitly demonstrating the EKS↔AKS cost contrast. Reach for the k3s variant when the
control-plane fee isn't justified.
