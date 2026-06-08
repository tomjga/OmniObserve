# Module: aws-ec2 — a single, agent-instrumented EC2 instance.
#
# STATUS: illustrative skeleton (design increment). Resource bodies are intentionally
# minimal and NOT wired to a configured provider yet — `tofu validate` is CI-deferred.
# This is the "1 EC2" half of Scenario A (ec2+vm) and the AWS side of Scenario C.
#
# Design choices baked in:
#   - Smallest burstable Graviton type by default (t4g.small) — cheap, ARM, plenty for a demo.
#   - user_data installs the observability agent (Grafana Alloy / OTel) so the box ships
#     metrics+logs+traces to the OmniObserve collector from first boot. See modules/observability-agent.
#   - A minimal security group: SSH (lock to your IP) + the app port + the agent's OTLP port.

terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "name" {
  type    = string
  default = "omniobserve-ec2"
}
variable "region" {
  type    = string
  default = "us-east-1"
}
variable "instance_type" {
  type    = string
  default = "t4g.small" # ~$0.0168/h on-demand (arm64 / Graviton)
}
variable "use_spot" {
  type    = bool
  default = true # ~70% cheaper; acceptable for a demo workload
}
variable "app_port" {
  type    = number
  default = 8080
}
variable "ssh_cidr" {
  type    = string
  default = "0.0.0.0/0" # OVERRIDE in tfvars: lock to your /32
}
variable "agent_user_data" {
  type    = string
  default = "" # cloud-init from modules/observability-agent
}
variable "tags" {
  type    = map(string)
  default = {}
}

# data "aws_ami" "al2023" { ... }   # latest Amazon Linux 2023 (arm64) — filled in apply phase
# resource "aws_security_group" "this" { ... ingress: ssh_cidr:22, app_port, 4318 (otlp) ... }
# resource "aws_instance" "this" {
#   ami           = data.aws_ami.al2023.id
#   instance_type = var.instance_type
#   user_data     = var.agent_user_data
#   tags          = merge({ Name = var.name, project = "omniobserve" }, var.tags)
# }
# resource "aws_eip" "this" { instance = aws_instance.this.id }   # stable public IP for DNS failover

output "public_ip" {
  value       = "<eip>" # parked: real value after apply
  description = "Stable public IP for DNS failover routing"
}
output "summary" {
  value = "aws-ec2 ${var.instance_type} in ${var.region} (spot=${var.use_spot})"
}
