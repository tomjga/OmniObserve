# Scenario A — ec2-plus-vm : 1 AWS EC2 + 1 Azure VM, fronted by health-checked DNS.
#
# STATUS: illustrative composition (design increment). Wires the building-block modules into the
# cheapest cross-cloud HA topology. `tofu plan` here is what enable-ha.sh runs in dry-run.
#
# The headline demo: two plain IaaS boxes, one per cloud, same app, same observability agent,
# one DNS name in front. Kill either cloud and the survivor keeps serving. No managed K8s,
# so it's the cheapest way to prove the cross-cloud reliability idea.

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

module "agent_azure" {
  source         = "../../modules/observability-agent"
  backend        = "grafana"
  install_target = "vm"
  otlp_endpoint  = var.otlp_endpoint
}

module "ec2" {
  source          = "../../modules/aws-ec2"
  name            = "omniobserve-a-ec2"
  ssh_cidr        = var.ssh_cidr
  agent_user_data = module.agent_aws.cloud_init
}

module "vm" {
  source           = "../../modules/azure-vm"
  name             = "omniobserve-a-vm"
  ssh_cidr         = var.ssh_cidr
  agent_cloud_init = module.agent_azure.cloud_init
}

# DR layer (enable-dr.sh adds the var-file that flips routing on). Active-active by default.
module "failover" {
  source         = "../../modules/dns-failover"
  fqdn           = var.fqdn
  primary_ip     = module.ec2.public_ip
  secondary_ip   = module.vm.public_ip
  routing_policy = "active-active"
}

output "scenario" {
  value = "A ec2+vm | ${module.ec2.summary} + ${module.vm.summary} | ${module.failover.summary}"
}
