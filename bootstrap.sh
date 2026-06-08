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
  --set alertmanager.alertmanagerSpec.alertmanagerConfigMatcherStrategy.type=None \
  --wait --timeout 12m
# matcherStrategy=None: the operator otherwise injects a namespace= matcher into every
# AlertmanagerConfig route, which would stop the cross-cutting remediator from receiving
# alerts raised in other namespaces (e.g. the demo). We route on our explicit
# omniobserve_remediate label instead — see deploy/remediator/templates/alertmanagerconfig.yaml.

echo "==> Argo Rollouts controller"
helm upgrade --install argo-rollouts argo/argo-rollouts -n argo-rollouts --create-namespace \
  --wait --timeout 6m

echo "==> Build api-service image locally (k3s uses the docker runtime — no registry push needed)"
VERSION="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
echo "    version: $VERSION"
docker build -f "$ROOT/services/api-service/Dockerfile" --build-arg VERSION="$VERSION" -t omniobserve-api-service:local "$ROOT"

echo "==> Deploy api-service as a canary Rollout"
helm upgrade --install api-service "$ROOT/deploy/api-service" -n "$NS" \
  --set rollout.enabled=true \
  --set image.repository=omniobserve-api-service \
  --set image.tag=local \
  --set image.pullPolicy=IfNotPresent

echo "==> Build + deploy worker-service (steady synthetic traffic for SLO demos)"
docker build -f "$ROOT/services/worker-service/Dockerfile" --build-arg VERSION="$VERSION" -t omniobserve-worker-service:local "$ROOT"
helm upgrade --install worker-service "$ROOT/deploy/worker-service" -n "$NS" \
  --set image.repository=omniobserve-worker-service \
  --set image.tag=local \
  --set image.pullPolicy=IfNotPresent

echo "==> Build + deploy the remediator (control loop: Alertmanager webhook -> action + RCA)"
# Root context so the incident corpus is baked in for the RCA copilot.
docker build -f "$ROOT/remediator/Dockerfile" --build-arg VERSION="$VERSION" -t omniobserve-remediator:local "$ROOT"
helm upgrade --install remediator "$ROOT/deploy/remediator" -n "$NS" \
  --set image.repository=omniobserve-remediator \
  --set image.tag=local \
  --set image.pullPolicy=IfNotPresent

cat <<EOF

==> Done. The stack is up in namespace '$NS'.

Watch the rollout:
  kubectl argo rollouts get rollout api-service -n $NS --watch

Reach the service + Grafana (separate shells):
  kubectl -n $NS port-forward svc/api-service 8080:8080
  kubectl -n $NS port-forward svc/kps-grafana 3000:80   # admin / prom-operator

See the remediator react to an SLO breach (Phase 2):
  kubectl -n $NS logs deploy/remediator -f          # watch it receive alerts
  # drive errors so ApiServiceHighErrorRate fires and routes to the webhook:
  kubectl -n $NS run apierr --rm -it --restart=Never --image=curlimages/curl -- \
    sh -c 'while true; do curl -s -o /dev/null "http://api-service:8080/kpi/errors?error_rate=100"; done'

Run the auto-rollback demo: see demo/README.md
EOF
