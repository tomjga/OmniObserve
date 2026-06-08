# Local stack as IaC (OpenTofu)

The codified equivalent of `bootstrap.sh` + `bootstrap-telemetry.sh`: the whole OmniObserve
platform on a **local** Kubernetes cluster, as OpenTofu instead of shell. Builds the local
images, installs the Helm releases, applies the collector/secret/rules manifests, and points
flagd at its ConfigMap so the self-heal loop works.

## What it manages
| Resource | Replaces |
|---|---|
| `helm_release.kps` | kube-prometheus-stack (+ the matcher-strategy / remote-write flags) |
| `helm_release.argo_rollouts` | Argo Rollouts controller |
| `helm_release.tempo` | tempo-distributed |
| `helm_release.otel_demo` | the OpenTelemetry Demo workload |
| `helm_release.api_service` / `worker_service` / `remediator` | the in-house charts (local images) |
| `kubernetes_config_map.otelcol` / `kubectl_manifest.collector` | the OTel Collector |
| `kubernetes_secret.grafana_cloud` | Grafana Cloud OTLP creds (placeholder by default) |
| `kubectl_manifest.rules` / `datasource` | PrometheusRule + Tempo datasource |
| `null_resource.build_*` / `flagd_patch` | the docker builds + the flagd ConfigMap repoint |

## Use (on a CLEAN local cluster)
```bash
cd infrastructure/local-stack
tofu init
tofu apply -var kube_context=rancher-desktop
# real Grafana Cloud creds (optional): keep them in a gitignored cloud.auto.tfvars
#   grafana_cloud_endpoint      = "https://otlp-gateway-prod-...grafana.net/otlp"
#   grafana_cloud_authorization = "Basic <base64>"
```

> **This is a fresh-cluster path.** If you already ran the bootstrap scripts, the Helm
> releases exist and `apply` would conflict — either `tofu import` them first, or run this on a
> clean cluster. The shell scripts remain the quick day-to-day path; this is the reproducible,
> reviewable, single-command alternative (and the portfolio "it's all IaC" story).

## Notes
- Chart versions are unpinned (matching the scripts) — pin them for true reproducibility.
- Requires `kubectl`, `helm`, `docker`, and a local kube context. Guard rails: it only targets
  whatever `kube_context` you pass — point it at a local cluster only.
- State is local (gitignored).
