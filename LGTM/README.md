# LGTM Stack

Local Grafana, Loki, Tempo, and Prometheus support files for OmniObserve.

The current telemetry architecture is:

```text
services + remediator + worker-service
  -> OTLP
  -> OTel Collector
  -> Prometheus / Tempo / Loki / optional Grafana Cloud
```

## What Is Current

- `../collector/otelcol-config.yaml` is the primary fan-out config for metrics, traces, and
  OTLP logs.
- `tempo-values.yaml`, `loki-values.yaml`, `prometheus-values.yaml`, and `grafana-values.yaml`
  are the maintained local chart values.
- `alloy/` contains an optional Grafana Alloy DaemonSet for Kubernetes pod logs to Loki.

## Grafana Agent Status

Grafana Agent Operator manifests were retired. They were stale, not used by the bootstrap
path, and duplicated the log-shipping concern now handled by optional Alloy. Do not install
`monitoring.grafana.com` Agent CRDs for this project.

## Optional Pod Logs With Alloy

If Loki is installed and you want stdout/stderr pod logs in Grafana Explore:

```bash
kubectl apply -f LGTM/alloy/k8s.yaml
kubectl -n monitoring rollout status ds/alloy
```

The Alloy config labels pod logs with namespace, pod, container, app, and job, then writes to
`loki-gateway` in the `monitoring` namespace.

## Day-To-Day Local Path

Use the bootstrap scripts from the repo root:

```bash
./bootstrap.sh
./bootstrap-telemetry.sh
```

Then open Grafana:

```bash
kubectl -n monitoring port-forward svc/kps-grafana 3000:80
```

Login is `admin / prom-operator` for kube-prometheus-stack's local default.
