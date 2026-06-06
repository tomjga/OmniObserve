terraform {
  required_version = ">= 1.6"
  required_providers {
    kubernetes = { source = "hashicorp/kubernetes", version = "~> 2.30" }
    helm       = { source = "hashicorp/helm", version = "~> 2.17" }
    kubectl    = { source = "gavinbunney/kubectl", version = "~> 1.19" } # applies CRD manifests at apply-time (no plan-time CRD check)
    null       = { source = "hashicorp/null", version = "~> 3.2" }
  }
}

provider "kubernetes" {
  config_path    = var.kubeconfig
  config_context = var.kube_context
}

provider "helm" {
  kubernetes {
    config_path    = var.kubeconfig
    config_context = var.kube_context
  }
}

provider "kubectl" {
  config_path      = var.kubeconfig
  config_context   = var.kube_context
  load_config_file = true
}
