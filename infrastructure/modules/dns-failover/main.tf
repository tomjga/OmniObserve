# Module: dns-failover — health-checked traffic routing across the two clouds (the DR layer).
#
# STATUS: illustrative skeleton (design increment). Enabled by enable-dr.sh. This is what turns
# "two boxes in two clouds" into actual HA/DR: a health check per endpoint plus a routing policy
# that withdraws an unhealthy endpoint so traffic lands on the survivor.
#
# Two interchangeable implementations (pick per your registrar / latency needs):
#   - AWS Route 53      : health checks + failover (or weighted/latency) records.
#   - Azure Traffic Mgr : Priority (active/passive) or Weighted (active/active) profiles.
# A cloud-agnostic alternative is documented in the design doc (e.g. Cloudflare LB) so the
# routing layer itself isn't a single-cloud dependency.
#
# routing_policy:
#   - active-active  -> both endpoints serve; weighted/round-robin; lowest RTO.
#   - active-passive -> primary serves; standby promoted on health-check failure; cheaper.

terraform {
  required_version = ">= 1.6"
}

variable "fqdn" {
  type    = string
  default = "app.example.com"
}
variable "primary_ip" {
  type    = string
  default = "" # from the AWS module output
}
variable "secondary_ip" {
  type    = string
  default = "" # from the Azure module output
}
variable "routing_policy" {
  type    = string
  default = "active-active"
  validation {
    condition     = contains(["active-active", "active-passive"], var.routing_policy)
    error_message = "routing_policy must be 'active-active' or 'active-passive'."
  }
}
variable "health_check_path" {
  type    = string
  default = "/healthz"
}
variable "provider_impl" {
  type    = string
  default = "route53" # route53 | traffic-manager | cloudflare
}

# resource "aws_route53_health_check" "primary" / "secondary" { fqdn ... path = health_check_path }
# resource "aws_route53_record" "this" { failover_routing_policy { type = "PRIMARY"/"SECONDARY" } }

output "endpoint" {
  value = var.fqdn
}
output "summary" {
  value = "dns-failover ${var.provider_impl} ${var.routing_policy} -> ${var.fqdn}"
}
