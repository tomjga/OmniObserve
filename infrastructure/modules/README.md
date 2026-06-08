# Modules — reusable building blocks

Small, single-purpose OpenTofu modules that the `scenarios/` compose. Each module's `main.tf`
header documents its design choices; this is the index.

| Module | Role | Notes |
|--------|------|-------|
| [`aws-ec2`](./aws-ec2) | One EC2 instance + SG + EIP, agent in user_data | `t4g.small`, spot by default |
| [`azure-vm`](./azure-vm) | One Azure Linux VM + NSG + public IP | `Standard_B2s`, agent via cloud-init |
| [`aws-eks`](./aws-eks) | Minimal EKS cluster | control plane ~$73/mo; k3s-on-EC2 = near-$0 variant |
| [`azure-aks`](./azure-aks) | Minimal AKS cluster | control plane **Free** tier |
| [`observability-agent`](./observability-agent) | Telemetry shipper (Grafana Alloy / OTel) | one config contract, VM *or* K8s DaemonSet; swappable backend |
| [`dns-failover`](./dns-failover) | Health-checked cross-cloud routing (DR) | Route 53 / Traffic Manager / Cloudflare |

> **Status:** illustrative skeletons for the design increment — resource bodies are minimal and
> not yet wired to a configured provider. `tofu fmt/validate` is CI-deferred (see the design doc).
> The composition logic, variables, outputs, and cost/observability decisions are real.
