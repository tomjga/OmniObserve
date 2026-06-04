#!/usr/bin/env bash
set -euo pipefail

# OmniObserve — local Phase 1 bootstrap.
# Stands up the SLO-gated auto-rollback demo on a clean LOCAL cluster
# (Rancher Desktop / kind / k3d / minikube / docker-desktop):
#
#   - kube-prometheus-stack  (Prometheus Operator + Prometheus + Grafana + Alertmanager)
#   - Argo Rollouts          (canary controller)
#   - api-service            (canary Rollout + ServiceMonitor + SLO AnalysisTemplate)
#
# Re-runnable (everything is `helm upgrade --install`). Run from the repo root.
# Override the namespace with NS=... ; skip the local-context guard with FORCE=1.

NS="${NS:-monitoring}"
ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "==> Preflight"
for b in kubectl helm docker; do command -v "$b" >/dev/null || { echo "ERROR: $b not found"; exit 1; }; done
docker info >/dev/null 2>&1 || { echo "ERROR: docker daemon not running (start Rancher Desktop)"; exit 1; }
CTX="$(kubectl config current-context)"
echo "    context:   $CTX"
echo "    namespace: $NS"
case "$CTX" in
  rancher-desktop|kind-*|k3d-*|minikube|docker-desktop) : ;;
  *) [ "${FORCE:-0}" = "1" ] || { echo "ERROR: '$CTX' doesn't look local. Re-run with FORCE=1 if you're sure."; exit 1; } ;;
esac

echo "==> Helm repos"
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
helm repo add argo https://argoproj.github.io/argo-helm >/dev/null 2>&1 || true
helm repo update >/dev/null
kubectl create namespace "$NS" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

echo "==> kube-prometheus-stack (this pulls several images — first run is slow)"
helm upgrade --install kps prometheus-community/kube-prometheus-stack -n "$NS" \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.enableRemoteWriteReceiver=true \
  --set prometheus.prometheusSpec.retention=2h \
  --set prometheus.prometheusSpec.resources.requests.memory=400Mi \
  --wait --timeout 12m

echo "==> Argo Rollouts controller"
helm upgrade --install argo-rollouts argo/argo-rollouts -n argo-rollouts --create-namespace \
  --wait --timeout 6m

echo "==> Build api-service image locally (k3s uses the docker runtime — no registry push needed)"
docker build -t omniobserve-api-service:local "$ROOT/application"

echo "==> Deploy api-service as a canary Rollout"
helm upgrade --install api-service "$ROOT/deploy/api-service" -n "$NS" \
  --set rollout.enabled=true \
  --set image.repository=omniobserve-api-service \
  --set image.tag=local \
  --set image.pullPolicy=IfNotPresent

cat <<EOF

==> Done. The stack is up in namespace '$NS'.

Watch the rollout:
  kubectl argo rollouts get rollout api-service -n $NS --watch

Reach the service + Grafana (separate shells):
  kubectl -n $NS port-forward svc/api-service 8080:8080
  kubectl -n $NS port-forward svc/kps-grafana 3000:80   # admin / prom-operator

Run the auto-rollback demo: see demo/README.md
EOF
