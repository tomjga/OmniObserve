# Multi-Cloud HA/DR — Architecture & Decisions

> **Status:** design + scaffolding increment (2026-06). Dry-run only; nothing is provisioned.
> This document is the source of truth; the `infrastructure/` modules, scenarios and scripts are
> the front-loaded design of OmniObserve **Phase 2** (which already targets AWS IaC, RTO/RPO docs,
> a weekly DR drill, and a < $20/mo budget).

## 1. Goal & non-goals

**Goal.** Demonstrate, at a Systems-Architect level, how to run a service **reliably across two
clouds (AWS + Azure)** and recover from the loss of an entire provider — provisioned **on demand**
with one command (`enable-ha` / `enable-dr`) via **OpenTofu**, observable end-to-end, and cheap
enough to spin up for a demo and tear down.

**Non-goals (this increment).**
- No real `tofu apply`, no live cloud resources, no spend. Scripts are dry-run by default and
  refuse to apply without explicit flags *and* authenticated tooling.
- No production data layer. The demo app is treated as stateless; cross-cloud data replication is
  described as the next hard problem, not solved here.
- Not a Kubernetes-federation project. We deliberately keep heterogeneous, simple units to *teach
  the trade-offs* rather than hide them behind a single abstraction.

## 2. Definitions (so HA and DR don't blur)

- **High Availability (HA):** redundancy that keeps the service up through the loss of a *component*
  (a host, an AZ, a pod). Measured as uptime / availability SLO. Mechanisms here: two compute units
  in two clouds, health-checked DNS, and (on the K8s scenarios) pod self-healing + autoscaling.
- **Disaster Recovery (DR):** the ability to recover from the loss of a *whole site/region/provider*.
  Measured as **RTO** (how long to recover) and **RPO** (how much data you can lose). Mechanisms:
  cross-cloud standby, backup/replication, failover routing, and a rehearsed **DR drill**.
- **Active-active:** both endpoints serve simultaneously (lowest RTO, higher cost/complexity).
- **Active-passive:** primary serves, standby is promoted on failure (cheaper, slightly higher RTO).

## 3. The three scenarios (and why heterogeneous)

Each scenario is **one unit per cloud**, chosen to expose a specific trade-off. Full diagrams,
pros/cons and pricing tables are in each scenario's README.

| | Scenario | Topology | Core lesson | ~$/mo* |
|-|----------|----------|-------------|--------|
| **A** | `ec2+vm`  | 1 AWS EC2 + 1 Azure VM | cheapest cross-cloud HA on plain IaaS; you own orchestration | ~$15–30 |
| **B** | `ec2+aks` | 1 AWS EC2 + Azure AKS  | IaaS vs managed-K8s; AKS control plane is **Free** | ~$90–140 |
| **C** | `vm+eks`  | 1 Azure VM + AWS EKS   | mirror of B; EKS control plane bills **~$73/mo** | ~$95–140 |

*Approximate, 2026-06, on-demand, smallest burstable types (`t4g.small` / `Standard_B2s`),
us-east-1 / eastus. Spot and the k3s-on-EC2 variant (Scenario C README) cut this substantially.
DR adds standby + replication, so it costs more than the HA figure.*

**Why not one uniform stack?** A Staff/Architect interview is won on *trade-off reasoning*. Putting
a hand-operated VM next to a self-healing cluster, and AKS's free control plane next to EKS's paid
one, makes the cost/operability/reliability trade-offs concrete and defensible.

### Decision matrix
| Concern | A: ec2+vm | B: ec2+aks | C: vm+eks |
|---------|-----------|------------|-----------|
| Self-healing (pod reschedule) | ❌ host-only | ✅ on AKS side | ✅ on EKS side |
| Autoscaling | ❌ | ✅ (HPA) | ✅ (HPA) |
| Control-plane cost | $0 | $0 (AKS Free) | ~$73/mo (EKS) |
| Cross-cloud outage survival | ✅ | ✅ | ✅ |
| Operational simplicity | ✅✅ | ➖ (VM + cluster) | ➖ (VM + cluster) |
| Cheapest | ✅ | ➖ | ❌ (use k3s variant) |
| Best "managed K8s on org's primary cloud" story | ❌ | ➖ | ✅ |

## 4. OpenTofu design

**Tool choice — OpenTofu.** Linux-Foundation, MPL-licensed Terraform fork; fits OmniObserve's
OSS / vendor-neutral ethos and is a clean portfolio signal. The HCL is Terraform-compatible, so
`scripts/_tofu.sh` auto-detects `tofu` and falls back to `terraform`.

**Layering.**
```
scripts/enable-ha.sh ─┐
scripts/enable-dr.sh ─┼─► scenarios/<topology>/main.tf ─► modules/* (compute, agent, dns-failover)
scripts/destroy.sh   ─┘            (envs/*.tfvars select the posture)
```
- **modules/** — single-purpose, reusable (`aws-ec2`, `azure-vm`, `aws-eks`, `azure-aks`,
  `observability-agent`, `dns-failover`). Compute modules accept an `agent_*` input so every box
  ships telemetry from first boot.
- **scenarios/** — thin compositions that wire modules into a topology and expose a `scenario` output.
- **envs/** — `single` / `ha` / `dr` tfvars select the posture (committed only as `*.tfvars.example`).

**State backend.** Local for now (demo). For real use: **S3 + DynamoDB lock** (AWS) or an **Azure
Storage** container, one state per scenario, documented but not enabled this increment.

**CI (deferred).** A `tofu fmt -check` + `tofu validate` + `tofu plan` (no apply) job, plus
`tflint`/`checkov` for policy. Not added yet because no IaC binary is installed locally — flagged so
it's an obvious next step, not an omission.

## 5. The `enable-ha` / `enable-dr` command UX

```bash
enable-ha.sh --scenario <A|B|C | ec2+vm|ec2+aks|vm+eks> --cloud <aws|azure|both> [--dry-run|--apply --auto-approve]
enable-dr.sh --scenario ...                              # adds standby + dns-failover + RTO/RPO + drill
destroy.sh   --scenario ...                              # tear down
```
**Safety model (mirrors the dashboard's wallet bridge — preview, then confirm):**
1. `--dry-run` is the **default** → `tofu plan` only; prints the exact commands and the plan.
2. `--apply` alone is rejected; you must add `--auto-approve` (guards against fat-finger provisioning).
3. Even with both, the script **bails unless** an IaC binary *and* the relevant cloud CLI are
   present/authenticated. On this machine (no tofu/aws/az) it prints install + auth guidance.

This is the CLI that the dashboard's **Reliability** tab surfaces (copy-to-run today; a future
auth-gated `vite-plugin-infra` bridge will invoke it with the same preview→confirm flow).

## 6. Observability — how telemetry leaves every unit

The future-proofing requirement: **k8s/VM agents that pull metrics/traces/logs** like the popular
stacks. Design principle — **collect once through an OpenTelemetry-compatible agent, keep the
backend swappable.** That's exactly the OmniObserve collector pattern (`collector/otelcol-config.yaml`)
extended to remote cloud units.

- **VM / EC2:** the `observability-agent` module renders cloud-init that installs **Grafana Alloy /
  OTel Collector** as a systemd unit → OTLP to the OmniObserve collector.
- **K8s (EKS/AKS):** the same agent as a **DaemonSet via Helm** (one collector per node) +
  kube-state-metrics. Same OTLP contract, so the backend doesn't care where data came from.

### Agent / backend comparison
Default = **Grafana stack (Alloy → LGTM)**: OSS, $0, vendor-neutral, already in this repo. The
matrix is *why* — and why the OTel-agent indirection matters (swap the column, keep the workload):

| | **Grafana (Alloy/LGTM)** ★default | **Datadog** | **New Relic** | **Elastic (ELK)** |
|---|---|---|---|---|
| Cost model | OSS / self-host (infra only) | per-host + per-feature, usage | per-GB ingest + per-user | OSS or Elastic Cloud (per-resource) |
| Vendor lock-in | **Low** (OSS, OTLP-native) | High (proprietary agent/UI) | High | Medium (OSS core, paid features) |
| k8s agent | Alloy/OTel **DaemonSet** (Helm) | Datadog Agent DaemonSet + Operator | NR infra agent + nri-bundle | Elastic Agent / Beats DaemonSet |
| Signals | metrics·logs·traces·profiles | metrics·logs·traces·RUM·security | metrics·logs·traces·RUM | logs·metrics·traces (APM) |
| Hosting | self-host or Grafana Cloud | SaaS only | SaaS only | self-host or Elastic Cloud |
| Best when | cost-sensitive, OSS, no lock-in | want turnkey SaaS, budget exists | turnkey SaaS, consumption pricing | log-heavy / search-centric |
| Watch-outs | you operate the backend | cost scales sharply with hosts/usage | ingest cost spikes | cluster ops + index lifecycle |

**Architectural payoff:** because every unit emits **OTLP to an OTel-compatible agent**, switching
from Grafana to Datadog/New Relic/Elastic is an *exporter/back-end change in the collector* — the
EC2 boxes, VMs and clusters don't change. No re-instrumentation, no lock-in.

## 7. Cost guardrails

- Smallest burstable types (`t4g.small` / `Standard_B2s`); **spot/preemptible** where tolerable.
- Prefer **AKS Free tier** and the **k3s-on-EC2** variant to dodge control-plane fees.
- **Tear down by default** — `destroy.sh` is first-class; demos are ephemeral.
- Budget alerts (AWS Budgets / Azure Cost Management) before any real apply; keep the OmniObserve
  target of **< $20/mo** for the cheap scenarios.

## 8. Security

- **No secrets in git:** `*.tfvars`, `*.tfstate*`, `.terraform/`, lock files already gitignored
  (repo-root `.gitignore`). Only `*.tfvars.example` (placeholder values) is committed.
- **Credentials:** AWS via SSO/OIDC (GitHub OIDC → role assumption for CI, no long-lived keys);
  Azure via `az login` / workload identity. The scripts never read or store credentials.
- **Network:** SSH locked to your `/32` (`ssh_cidr`), not `0.0.0.0/0`, before any apply.
- **State:** remote backend (encrypted, locked) before multi-operator or real use.

## 9. Roadmap to real apply (deferred, in order)

1. Install OpenTofu + AWS/Azure CLIs; wire providers; `tofu validate` green in CI.
2. Fill module resource bodies (AMI/image lookups, SGs/NSGs, instances, clusters, Route 53/TM).
3. Remote state (S3+DynamoDB / Azure Storage) per scenario.
4. First real **Scenario A** apply on a budget, capture the demo, `destroy`.
5. Auth-gated **`vite-plugin-infra`** bridge so the dashboard's *Enable HA/DR* buttons run the
   scripts with preview→confirm (same model as the wallet bridge).
6. Observability agents shipping from the cloud units into the OmniObserve collector; SLO dashboards
   spanning both clouds; scheduled **weekly DR drill** (`destroy` standby → verify failover → re-apply).
