# Module: aws-eks — a minimal EKS cluster (managed Kubernetes on AWS).
#
# STATUS: illustrative skeleton (design increment). Used as the AWS side of Scenario C (vm+eks).
#
# Cost reality (call it out in the demo): EKS charges a flat ~$0.10/hour (~$73/mo) for the
# control plane PER CLUSTER, on top of the worker nodes. That's the headline trade-off vs the
# plain VM/EC2 scenarios. For a near-$0 variant, prefer the k3s-on-EC2 path (see README) which
# self-manages the control plane on a single t4g instance.
#
# The observability story on K8s changes shape: instead of a per-VM agent in user_data, you run
# the agent as a DaemonSet (one collector pod per node) via Helm — see modules/observability-agent.

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
  default = "omniobserve-eks"
}
variable "region" {
  type    = string
  default = "us-east-1"
}
variable "k8s_version" {
  type    = string
  default = "1.30"
}
variable "node_instance_type" {
  type    = string
  default = "t4g.small"
}
variable "desired_nodes" {
  type    = number
  default = 2
}
variable "use_spot" {
  type    = bool
  default = true
}
variable "tags" {
  type    = map(string)
  default = {}
}

# Recommended: compose the community module rather than hand-rolling VPC + control plane.
# module "eks" {
#   source          = "terraform-aws-modules/eks/aws"
#   cluster_name    = var.name
#   cluster_version = var.k8s_version
#   eks_managed_node_groups = { default = { instance_types = [var.node_instance_type], ... } }
# }

output "cluster_name" {
  value = var.name
}
output "summary" {
  value = "aws-eks ${var.name} v${var.k8s_version}, ${var.desired_nodes}x ${var.node_instance_type} (control plane ~$73/mo)"
}
