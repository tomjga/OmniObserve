#!/usr/bin/env bash
set -euo pipefail

# OmniObserve — real-telemetry layer (Phase 1.5).
# Run AFTER bootstrap.sh, from the repo root. Adds:
#   - Tempo                (distributed traces backend)
#   - OTel Collector       (our collector/otelcol-config.yaml, in-cluster)
#   - Grafana Tempo datasource
#   - the OpenTelemetry Demo as a real observed workload, routed into our collector
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

helm repo add grafana https://grafana.github.io/helm-charts >/dev/null 2>&1 || true
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts >/dev/null 2>&1 || true
helm repo update >/dev/null

echo "==> Tempo (traces backend)"
helm upgrade --install tempo grafana/tempo -n "$NS" --wait --timeout 6m

echo "==> OTel Collector (config from collector/otelcol-config.yaml)"
kubectl create configmap otelcol-config -n "$NS" \
  --from-file=config.yaml="$ROOT/collector/otelcol-config.yaml" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n "$NS" -f "$ROOT/collector/k8s/collector.yaml"
kubectl rollout restart deployment/otelcol -n "$NS" >/dev/null 2>&1 || true

echo "==> Grafana Tempo datasource"
kubectl apply -f "$ROOT/workloads/otel-demo/grafana-tempo-datasource.yaml"

echo "==> OpenTelemetry Demo (real workload -> our collector)"
helm upgrade --install otel-demo open-telemetry/opentelemetry-demo \
  -n otel-demo --create-namespace -f "$ROOT/workloads/otel-demo/values.yaml" \
  --wait --timeout 12m \
  || echo "    NOTE: demo install returned non-zero — check pods; chart value keys vary by version (helm show values open-telemetry/opentelemetry-demo)."

cat <<EOF

==> Telemetry layer up. Verify in Grafana:
  kubectl -n $NS port-forward svc/kps-grafana 3000:80      # login admin / prom-operator
  # Grafana -> Explore -> Tempo -> Search -> recent traces (frontend -> cartservice -> ...)

Inject a realistic fault (demo flagd feature flags), then watch the error signal:
  kubectl -n otel-demo get configmap          # flagd-config holds the feature flags
EOF
