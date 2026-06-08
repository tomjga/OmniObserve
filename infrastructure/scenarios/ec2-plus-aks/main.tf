# Scenario B — ec2-plus-aks : 1 AWS EC2 + Azure AKS.
#
# STATUS: illustrative composition (design increment). Pairs a plain IaaS box on AWS with managed
# Kubernetes on Azure to teach the asymmetry: a single VM you hand-operate vs a scheduler that
# self-heals pods and scales — and what that costs.

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

module "agent_aws" {
  source         = "../../modules/observability-agent"
  backend        = "grafana"
  install_target = "vm"
  otlp_endpoint  = var.otlp_endpoint
}

# On K8s the agent is a DaemonSet (one collector per node), installed via Helm.
module "agent_aks" {
  source         = "../../modules/observability-agent"
  backend        = "grafana"
  install_target = "k8s"
  otlp_endpoint  = var.otlp_endpoint
}

module "ec2" {
  source          = "../../modules/aws-ec2"
  name            = "omniobserve-b-ec2"
  ssh_cidr        = var.ssh_cidr
  agent_user_data = module.agent_aws.cloud_init
}

module "aks" {
  source = "../../modules/azure-aks"
  name   = "omniobserve-b-aks"
}

module "failover" {
  source         = "../../modules/dns-failover"
  fqdn           = var.fqdn
  primary_ip     = module.ec2.public_ip
  secondary_ip   = "<aks-ingress-ip>" # parked: AKS LoadBalancer ingress after apply
  routing_policy = "active-passive"
}

output "scenario" {
  value = "B ec2+aks | ${module.ec2.summary} + ${module.aks.summary} | ${module.failover.summary}"
}
