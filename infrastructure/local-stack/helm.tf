# Helm releases — the codified equivalent of bootstrap.sh + bootstrap-telemetry.sh.
# Chart versions are intentionally unpinned to match the scripts; pin them for reproducibility.

resource "helm_release" "kps" {
  name             = "kps"
  repository       = "https://prometheus-community.github.io/helm-charts"
  chart            = "kube-prometheus-stack"
  namespace        = var.namespace
  create_namespace = true
  wait             = true
  timeout          = 720

  # Let our ServiceMonitors/PrometheusRules be discovered cluster-wide.
  set {
    name  = "prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues"
    value = "false"
  }
  set {
    name  = "prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues"
    value = "false"
  }
  set {
    name  = "prometheus.prometheusSpec.enableRemoteWriteReceiver"
    value = "true"
  }
  set {
    name  = "prometheus.prometheusSpec.retention"
    value = "2h"
  }
  set {
    name  = "prometheus.prometheusSpec.resources.requests.memory"
    value = "400Mi"
  }
  # Don't inject a namespace= matcher into AlertmanagerConfigs — the remediator routes on the
  # explicit omniobserve_remediate label across namespaces (see INC-2026-0006).
  set {
    name  = "alertmanager.alertmanagerSpec.alertmanagerConfigMatcherStrategy.type"
    value = "None"
  }
}

resource "helm_release" "argo_rollouts" {
  name             = "argo-rollouts"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-rollouts"
  namespace        = "argo-rollouts"
  create_namespace = true
  wait             = true
  timeout          = 360
}

resource "helm_release" "tempo" {
  name       = "tempo"
  repository = "https://grafana-community.github.io/helm-charts"
  chart      = "tempo-distributed"
  namespace  = var.namespace
  wait       = true
  timeout    = 480
  values     = [file("${var.repo_root}/workloads/otel-demo/tempo-distributed-values.yaml")]

  depends_on = [helm_release.kps]
}

resource "helm_release" "otel_demo" {
  name             = "otel-demo"
  repository       = "https://open-telemetry.github.io/opentelemetry-helm-charts"
  chart            = "opentelemetry-demo"
  namespace        = "otel-demo"
  create_namespace = true
  wait             = true
  timeout          = 720
  values           = [file("${var.repo_root}/workloads/otel-demo/values.yaml")]

  depends_on = [kubectl_manifest.collector]
}

# In-house charts (local paths). Images are built by null_resource.build_* below.
resource "helm_release" "api_service" {
  name      = "api-service"
  chart     = "${var.repo_root}/deploy/api-service"
  namespace = var.namespace

  set {
    name  = "rollout.enabled"
    value = "true"
  }
  set {
    name  = "image.repository"
    value = "omniobserve-api-service"
  }
  set {
    name  = "image.tag"
    value = "local"
  }
  set {
    name  = "image.pullPolicy"
    value = "IfNotPresent"
  }

  depends_on = [helm_release.kps, helm_release.argo_rollouts, null_resource.build_api_service]
}

resource "helm_release" "remediator" {
  name      = "remediator"
  chart     = "${var.repo_root}/deploy/remediator"
  namespace = var.namespace

  set {
    name  = "image.repository"
    value = "omniobserve-remediator"
  }
  set {
    name  = "image.tag"
    value = "local"
  }
  set {
    name  = "image.pullPolicy"
    value = "IfNotPresent"
  }

  depends_on = [helm_release.kps, null_resource.build_remediator]
}
