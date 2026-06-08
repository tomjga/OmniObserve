# Module: azure-aks — a minimal AKS cluster (managed Kubernetes on Azure).
#
# STATUS: illustrative skeleton (design increment). Used as the Azure side of Scenario B (ec2+aks).
#
# Cost note (the contrast vs EKS, worth a callout in the demo): AKS's control plane is FREE on
# the default Free tier (you pay only for nodes); the optional Standard/uptime-SLA tier adds a
# per-cluster hourly charge. So "EC2 + AKS" can be cheaper than "VM + EKS" purely on control-plane
# economics — a nice, concrete cross-cloud pricing lesson.
#
# Observability: agent runs as a DaemonSet via Helm (same as EKS) — see modules/observability-agent.

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
  default = "omniobserve-aks"
}
variable "location" {
  type    = string
  default = "eastus"
}
variable "k8s_version" {
  type    = string
  default = "1.30"
}
variable "node_size" {
  type    = string
  default = "Standard_B2s"
}
variable "node_count" {
  type    = number
  default = 2
}
variable "sku_tier" {
  type    = string
  default = "Free" # "Standard" adds the uptime-SLA control-plane charge
}
variable "tags" {
  type    = map(string)
  default = {}
}

# resource "azurerm_kubernetes_cluster" "this" {
#   name                = var.name
#   kubernetes_version  = var.k8s_version
#   sku_tier            = var.sku_tier
#   default_node_pool { vm_size = var.node_size  node_count = var.node_count }
#   identity { type = "SystemAssigned" }
# }

output "cluster_name" {
  value = var.name
}
output "summary" {
  value = "azure-aks ${var.name} v${var.k8s_version}, ${var.node_count}x ${var.node_size} (control plane Free tier)"
}
