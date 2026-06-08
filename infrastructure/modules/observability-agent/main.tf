# Module: observability-agent — the telemetry shipper installed on every compute unit.
#
# STATUS: illustrative skeleton (design increment). This is the future-proofing the user asked
# for: a single, swappable place that defines HOW metrics/traces/logs leave each EC2 box, Azure
# VM, or K8s node and reach a backend.
#
# Default (this project): Grafana Alloy / OpenTelemetry Collector, OSS, $0, vendor-neutral —
# consistent with the OmniObserve collector (collector/otelcol-config.yaml). The whole point of
# routing through an OTel-compatible agent is that the *backend* (Grafana LGTM today; Datadog /
# New Relic / Elastic tomorrow) can change WITHOUT touching the workload — see the comparison
# matrix in ../../docs/MULTICLOUD-HA-DR.md.
#
# Two install shapes, one config contract:
#   - VM / EC2  -> rendered into user_data / cloud-init (systemd unit running the agent).
#   - K8s       -> a Helm release deploying the agent as a DaemonSet (one collector per node).

terraform {
  required_version = ">= 1.6"
}

variable "backend" {
  type    = string
  default = "grafana" # grafana | datadog | newrelic | elastic — selects the exporter block
}
variable "otlp_endpoint" {
  type    = string
  default = "" # e.g. the OmniObserve collector's OTLP ingest (collector:4317)
}
variable "install_target" {
  type    = string
  default = "vm" # vm | k8s — chooses cloud-init vs Helm DaemonSet
  validation {
    condition     = contains(["vm", "k8s"], var.install_target)
    error_message = "install_target must be 'vm' or 'k8s'."
  }
}
variable "extra_labels" {
  type    = map(string)
  default = {}
}

# locals { user_data = templatefile("${path.module}/templates/alloy-cloud-init.yaml.tftpl", { ... }) }
# Exposes a ready-to-attach cloud-init string for the VM/EC2 modules to drop into user_data.

output "cloud_init" {
  value       = "# cloud-init: install + run agent (backend=${var.backend}) -> ${var.otlp_endpoint}"
  description = "Parked: rendered cloud-init for VM/EC2 modules (real templatefile() output after apply)"
}
output "summary" {
  value = "observability-agent backend=${var.backend} target=${var.install_target}"
}
