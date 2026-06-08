# OmniObserve — Multi-Cloud HA/DR Infrastructure

On-demand **High-Availability** and **Disaster-Recovery** infrastructure across **AWS + Azure**,
driven by **OpenTofu** and two commands:

```bash
scripts/enable-ha.sh --scenario ec2+vm  --cloud both     # stand up the HA pair (dry-run by default)
scripts/enable-dr.sh --scenario ec2+vm  --cloud both     # layer on the DR / failover (dry-run by default)
scripts/destroy.sh   --scenario ec2+vm  --cloud both     # tear it down (stay cheap)
```

> **Status: design + scaffolding increment.** Everything here is **dry-run only** — the scripts
> default to `tofu plan` and refuse to `apply` without `--apply --auto-approve` *and*
> present-and-authenticated tooling (none installed on this machine yet). No cloud resources are
> created, no credentials are read. This is the front-loaded design of OmniObserve **Phase 2**.
> Full rationale, scenario trade-offs, pricing, RTO/RPO and the observability-agent comparison
> live in **[docs/MULTICLOUD-HA-DR.md](./docs/MULTICLOUD-HA-DR.md)**.

## Layout

```
infrastructure/
├── docs/MULTICLOUD-HA-DR.md   # the architecture & decision doc (start here)
├── modules/                   # reusable building blocks (aws-ec2, azure-vm, *-eks/aks, agent, dns-failover)
├── scenarios/                 # compositions = the demo topologies (A: ec2+vm, B: ec2+aks, C: vm+eks)
├── envs/                      # *.tfvars.example per posture (single / ha / dr) — copy to *.tfvars (gitignored)
├── scripts/                   # enable-ha / enable-dr / destroy + _tofu.sh helper
└── aws/                       # original Phase-2 stub (absorbed by modules/aws-ec2)
```

## The three scenarios

| | Scenario | Topology | Teaches | ~$/mo |
|-|----------|----------|---------|-------|
| **A** | [ec2+vm](./scenarios/ec2-plus-vm)   | 1 EC2 + 1 Azure VM      | cheapest cross-cloud HA (plain IaaS) | ~$15–30 |
| **B** | [ec2+aks](./scenarios/ec2-plus-aks) | 1 EC2 + Azure AKS       | IaaS vs managed-K8s; AKS free control plane | ~$90–140 |
| **C** | [vm+eks](./scenarios/vm-plus-eks)   | 1 Azure VM + AWS EKS    | mirror of B; EKS ~$73/mo control-plane fee | ~$95–140 |

Each scenario's README has the diagram, pros/cons, pricing table and RTO/RPO.

## Tooling

- **OpenTofu** is the standard (`brew install opentofu`). The scripts auto-detect `tofu` and fall
  back to `terraform` — the HCL is identical for both.
- Cloud CLIs for real applies: `aws` (AWS CLI / SSO / OIDC) and `az` (Azure CLI).
- The scripts detect missing tooling and print install/auth guidance instead of failing.

## Safety & secrets

- **No secrets in git:** `*.tfvars`, `*.tfstate*`, `.terraform/` are already gitignored (repo root
  `.gitignore`). Only `*.tfvars.example` (no real values) is committed.
- State backend is **local** for now; the design doc covers the S3+DynamoDB / Azure Storage remote
  backend for real use.
- Apply is guarded: `--apply` alone is rejected; you must add `--auto-approve`, and even then the
  scripts bail unless an IaC binary + the relevant cloud CLI are available.

See **[docs/MULTICLOUD-HA-DR.md](./docs/MULTICLOUD-HA-DR.md)** for the full design.
