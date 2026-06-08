# Module: azure-vm — a single, agent-instrumented Azure Linux VM.
#
# STATUS: illustrative skeleton (design increment). Mirrors modules/aws-ec2 on Azure so a
# scenario can place one unit per cloud. This is the "1 Azure VM" half of Scenario A (ec2+vm)
# and the Azure side of Scenario C (vm+eks).
#
# Design choices:
#   - Standard_B2s burstable by default (2 vCPU / 4 GiB) — Azure's cheap demo tier.
#   - cloud-init runs the same observability agent so telemetry shape matches the EC2 box.
#   - A public IP + NSG mirroring the AWS security group (SSH locked, app port, OTLP).

terraform {
  required_version = ">= 1.6"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0"
    }
  }
}

variable "name" {
  type    = string
  default = "omniobserve-vm"
}
variable "location" {
  type    = string
  default = "eastus"
}
variable "vm_size" {
  type    = string
  default = "Standard_B2s" # ~$0.0416/h pay-as-you-go
}
variable "app_port" {
  type    = number
  default = 8080
}
variable "ssh_cidr" {
  type    = string
  default = "0.0.0.0/0" # OVERRIDE in tfvars: lock to your /32
}
variable "agent_cloud_init" {
  type    = string
  default = "" # cloud-init from modules/observability-agent
}
variable "tags" {
  type    = map(string)
  default = {}
}

# resource "azurerm_resource_group" "this" { name = "${var.name}-rg" location = var.location }
# resource "azurerm_public_ip" "this" { allocation_method = "Static" sku = "Standard" }
# resource "azurerm_network_security_group" "this" { ... 22/ssh_cidr, app_port, 4318 ... }
# resource "azurerm_linux_virtual_machine" "this" {
#   size           = var.vm_size
#   custom_data    = base64encode(var.agent_cloud_init)
#   tags           = merge({ project = "omniobserve" }, var.tags)
# }

output "public_ip" {
  value       = "<public_ip>" # parked: real value after apply
  description = "Stable public IP for DNS failover routing"
}
output "summary" {
  value = "azure-vm ${var.vm_size} in ${var.location}"
}
