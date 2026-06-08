#!/usr/bin/env bash
set -euo pipefail

# OmniObserve — real-telemetry layer (Phase 1.5).
# Run AFTER bootstrap.sh, from the repo root. Adds:
#   - Tempo                (distributed traces backend)
#   - OTel Collector       (our collector/otelcol-config.yaml, in-cluster)
#   - Grafana Tempo datasource
#   - the OpenTelemetry Demo as a real observed workload, routed into our collector
#   - optional Grafana Alloy pod-log shipping to Loki (INSTALL_ALLOY=1)
#
# Loki (logs) is optional and left out here to keep the first run robust — the
# collector's logs pipeline will log export errors until Loki exists; traces +
# metrics flow regardless. NS=... overrides the namespace; FORCE=1 skips the guard.

NS="${NS:-monitoring}"
ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "==> Preflight"
for b in kubectl helm; do command -v "$b" >/dev/null || { echo "ERROR: $b not found"; exit 1; }; done
CTX="$(kubectl config current-context)"
echo "    context: $CTX   namespace: $NS"
case "$CTX" in
  rancher-desktop|kind-*|k3d-*|minikube|docker-desktop) : ;;
  *) [ "${FORCE:-0}" = "1" ] || { echo "ERROR: '$CTX' doesn't look local. FORCE=1 to override."; exit 1; } ;;
esac

# grafana-community hosts the maintained tempo-distributed chart (the single-binary
# grafana/tempo chart is deprecated). open-telemetry hosts the demo + collector charts.
helm repo add grafana-community https://grafana-community.github.io/helm-charts >/dev/null 2>&1 || true
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts >/dev/null 2>&1 || true
helm repo update >/dev/null

echo "==> Tempo (microservices mode, tempo-distributed)"
helm upgrade --install tempo grafana-community/tempo-distributed -n "$NS" \
  -f "$ROOT/workloads/otel-demo/tempo-distributed-values.yaml" --wait --timeout 8m

echo "==> OTel Collector (config from collector/otelcol-config.yaml)"
kubectl create configmap otelcol-config -n "$NS" \
  --from-file=config.yaml="$ROOT/collector/otelcol-config.yaml" \
  --dry-run=client -o yaml | kubectl apply -f -
# Grafana Cloud OTLP secret: use the real (gitignored) file if present, else seed a harmless
# placeholder so the collector starts (cloud exports just fail-and-retry; local pipelines fine).
if [ -f "$ROOT/collector/grafana-cloud-secret.yaml" ]; then
  kubectl apply -n "$NS" -f "$ROOT/collector/grafana-cloud-secret.yaml"
else
  kubectl create secret generic grafana-cloud-otlp -n "$NS" \
    --from-literal=endpoint="https://otlp-gateway.invalid/otlp" \
    --from-literal=authorization="Basic none" \
    --dry-run=client -o yaml | kubectl apply -f -
fi
kubectl apply -n "$NS" -f "$ROOT/collector/k8s/collector.yaml"
kubectl rollout restart deployment/otelcol -n "$NS" >/dev/null 2>&1 || true

if [ "${INSTALL_ALLOY:-0}" = "1" ]; then
  echo "==> Grafana Alloy (optional pod logs -> Loki)"
  kubectl apply -f "$ROOT/LGTM/alloy/k8s.yaml"
  kubectl -n "$NS" rollout status ds/alloy --timeout=180s || true
else
  echo "==> Grafana Alloy skipped (set INSTALL_ALLOY=1 after Loki exists if pod logs are needed)"
fi

echo "==> Grafana Tempo datasource"
kubectl apply -f "$ROOT/workloads/otel-demo/grafana-tempo-datasource.yaml"

echo "==> Demo SLO alert (feeds the remediator control loop)"
kubectl apply -f "$ROOT/workloads/otel-demo/prometheus-rules.yaml"

echo "==> OpenTelemetry Demo (real workload -> our collector)"
helm upgrade --install otel-demo open-telemetry/opentelemetry-demo \
  -n otel-demo --create-namespace -f "$ROOT/workloads/otel-demo/values.yaml" \
  --wait --timeout 12m \
  || echo "    NOTE: demo install returned non-zero — check pods; chart value keys vary by version (helm show values open-telemetry/opentelemetry-demo)."

echo "==> Point flagd at the ConfigMap directly (so the remediator's ConfigMap patch reloads live)"
# By default flagd serves a writable copy that an init container seeds ONCE from the
# ConfigMap, so ConfigMap edits never reach it. Mounting the ConfigMap where flagd watches
# makes flagd hot-reload on edits and push to consumers over their open streams — no
# restarts. This is what lets the remediator heal by patching the ConfigMap alone.
kubectl -n otel-demo patch deploy flagd --type json -p '[
  {"op":"replace","path":"/spec/template/spec/containers/0/volumeMounts/0/name","value":"config-ro"},
  {"op":"add","path":"/spec/template/spec/containers/0/volumeMounts/0/readOnly","value":true}
]' >/dev/null 2>&1 || echo "    NOTE: flagd patch skipped (check container/volumeMount layout for this demo version)."
kubectl -n otel-demo rollout status deploy/flagd --timeout=120s >/dev/null 2>&1 || true

cat <<EOF

==> Telemetry layer up. Verify in Grafana:
  kubectl -n $NS port-forward svc/kps-grafana 3000:80      # login admin / prom-operator
  # Grafana -> Explore -> Tempo -> Search -> recent traces (frontend -> cartservice -> ...)
  # Optional pod logs: INSTALL_ALLOY=1 ./bootstrap-telemetry.sh

Inject a realistic fault (demo flagd feature flags), then watch the error signal:
  kubectl -n otel-demo get configmap          # flagd-config holds the feature flags
EOF
