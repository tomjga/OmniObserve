# ---- Local image builds (TF can't build images natively; shell out, rebuild each apply) ----
resource "null_resource" "build_api_service" {
  count    = var.build_images ? 1 : 0
  triggers = { always = timestamp() }
  provisioner "local-exec" {
    command = "docker build -f '${abspath(var.repo_root)}/services/api-service/Dockerfile' -t omniobserve-api-service:local '${abspath(var.repo_root)}'"
  }
}

resource "null_resource" "build_worker_service" {
  count    = var.build_images ? 1 : 0
  triggers = { always = timestamp() }
  provisioner "local-exec" {
    command = "docker build -f '${abspath(var.repo_root)}/services/worker-service/Dockerfile' -t omniobserve-worker-service:local '${abspath(var.repo_root)}'"
  }
}

resource "null_resource" "build_remediator" {
  count    = var.build_images ? 1 : 0
  triggers = { always = timestamp() }
  provisioner "local-exec" {
    # Root context so the incident corpus is baked into the image for the RCA copilot.
    command = "docker build -f '${abspath(var.repo_root)}/remediator/Dockerfile' -t omniobserve-remediator:local '${abspath(var.repo_root)}'"
  }
}

# ---- Collector config (ConfigMap) + Grafana Cloud creds (Secret) ----
resource "kubernetes_config_map" "otelcol" {
  metadata {
    name      = "otelcol-config"
    namespace = var.namespace
  }
  data = {
    "config.yaml" = file("${var.repo_root}/collector/otelcol-config.yaml")
  }
  depends_on = [helm_release.kps]
}

resource "kubernetes_secret" "grafana_cloud" {
  metadata {
    name      = "grafana-cloud-otlp"
    namespace = var.namespace
  }
  data = {
    endpoint      = var.grafana_cloud_endpoint
    authorization = var.grafana_cloud_authorization
  }
  depends_on = [helm_release.kps]
}

# ---- Raw manifests applied at apply-time (kubectl provider → no plan-time CRD checks) ----
data "kubectl_file_documents" "collector" {
  content = file("${var.repo_root}/collector/k8s/collector.yaml")
}

resource "kubectl_manifest" "collector" {
  for_each           = data.kubectl_file_documents.collector.manifests
  yaml_body          = each.value
  override_namespace = var.namespace

  depends_on = [kubernetes_config_map.otelcol, kubernetes_secret.grafana_cloud]
}

data "kubectl_file_documents" "datasource" {
  content = file("${var.repo_root}/workloads/otel-demo/grafana-tempo-datasource.yaml")
}

resource "kubectl_manifest" "datasource" {
  for_each   = data.kubectl_file_documents.datasource.manifests
  yaml_body  = each.value
  depends_on = [helm_release.kps]
}

# PrometheusRule that feeds the remediator — needs the CRD from kube-prometheus-stack.
data "kubectl_file_documents" "rules" {
  content = file("${var.repo_root}/workloads/otel-demo/prometheus-rules.yaml")
}

resource "kubectl_manifest" "rules" {
  for_each   = data.kubectl_file_documents.rules.manifests
  yaml_body  = each.value
  depends_on = [helm_release.kps]
}

# ---- Point flagd at the ConfigMap so the remediator's patch reloads live (INC-2026-0007) ----
resource "null_resource" "flagd_patch" {
  triggers   = { demo = helm_release.otel_demo.id }
  depends_on = [helm_release.otel_demo]
  provisioner "local-exec" {
    command = <<-EOT
      kubectl --context ${var.kube_context} -n otel-demo patch deploy flagd --type json -p '[
        {"op":"replace","path":"/spec/template/spec/containers/0/volumeMounts/0/name","value":"config-ro"},
        {"op":"add","path":"/spec/template/spec/containers/0/volumeMounts/0/readOnly","value":true}
      ]' || true
      kubectl --context ${var.kube_context} -n otel-demo rollout status deploy/flagd --timeout=120s || true
    EOT
  }
}
