# Scenario C — vm-plus-eks : 1 Azure VM + AWS EKS.
#
# STATUS: illustrative composition (design increment). The mirror of Scenario B with the clouds
# swapped — plain IaaS on Azure, managed K8s on AWS — so the EKS-vs-AKS control-plane economics
# become a concrete, side-by-side lesson. A near-$0 variant swaps EKS for k3s-on-EC2 (see README).

terraform {
  required_version = ">= 1.6"
}

variable "fqdn" {
  type    = string
  default = "app.example.com"
}
variable "otlp_endpoint" {
  type    = string
  default = ""
}
variable "ssh_cidr" {
  type    = string
  default = "0.0.0.0/0"
}

module "agent_azure" {
  source         = "../../modules/observability-agent"
  backend        = "grafana"
  install_target = "vm"
  otlp_endpoint  = var.otlp_endpoint
}

module "agent_eks" {
  source         = "../../modules/observability-agent"
  backend        = "grafana"
  install_target = "k8s"
  otlp_endpoint  = var.otlp_endpoint
}

module "vm" {
  source           = "../../modules/azure-vm"
  name             = "omniobserve-c-vm"
  ssh_cidr         = var.ssh_cidr
  agent_cloud_init = module.agent_azure.cloud_init
}

module "eks" {
  source = "../../modules/aws-eks"
  name   = "omniobserve-c-eks"
}

module "failover" {
  source         = "../../modules/dns-failover"
  fqdn           = var.fqdn
  primary_ip     = "<eks-ingress-ip>" # parked: EKS LoadBalancer ingress after apply
  secondary_ip   = module.vm.public_ip
  routing_policy = "active-passive"
}

output "scenario" {
  value = "C vm+eks | ${module.vm.summary} + ${module.eks.summary} | ${module.failover.summary}"
}
