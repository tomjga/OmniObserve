variable "kubeconfig" {
  description = "Path to kubeconfig."
  type        = string
  default     = "~/.kube/config"
}

variable "kube_context" {
  description = "kube context (must be a LOCAL cluster)."
  type        = string
  default     = "rancher-desktop"
}

variable "namespace" {
  description = "Namespace for the platform components."
  type        = string
  default     = "monitoring"
}

variable "repo_root" {
  description = "Path to the OmniObserve repo root (charts, values, manifests are read from here)."
  type        = string
  default     = "../.."
}

variable "build_images" {
  description = "Build the local api-service + remediator images via docker before deploying."
  type        = bool
  default     = true
}

variable "grafana_cloud_endpoint" {
  description = "Grafana Cloud OTLP endpoint. Placeholder by default (cloud exports just fail-and-retry; local pipelines fine)."
  type        = string
  default     = "https://otlp-gateway.invalid/otlp"
}

variable "grafana_cloud_authorization" {
  description = "Grafana Cloud OTLP 'Basic <base64>' header. Set in a gitignored *.tfvars; placeholder by default."
  type        = string
  default     = "Basic none"
  sensitive   = true
}
