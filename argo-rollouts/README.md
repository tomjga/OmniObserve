# Argo Rollouts (progressive delivery)

Phase 1 converts `api-service` from a Deployment to an Argo **Rollout** with a canary
strategy, gated by an `AnalysisTemplate` that queries Prometheus. A bad version that
breaches the error-rate SLO is **auto-aborted and rolled back**.

## Install the controller (once per cluster)

```bash
kubectl create namespace argo-rollouts
kubectl apply -n argo-rollouts \
  -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml

# kubectl plugin to watch/promote/abort rollouts:
#   brew install argoproj/tap/kubectl-argo-rollouts
# (or download from the Argo Rollouts releases page)
```

## Deploy api-service as a Rollout

```bash
helm upgrade --install api-service deploy/api-service -n monitoring \
  --set rollout.enabled=true
```

This renders a `Rollout` + an `AnalysisTemplate` (`api-service-slo`) instead of a
Deployment. The canary steps and SLO gate are configured under `rollout.*` in the
chart's [values.yaml](../deploy/api-service/values.yaml):

- **steps**: 25% → pause → 50% → pause → 100%
- **analysis**: background run from step 1; aborts when the 5xx ratio query exceeds
  `errorRatioThreshold` (0.05) more than `failureLimit` times.

Watch a rollout:

```bash
kubectl argo rollouts get rollout api-service -n monitoring --watch
```

See [../demo/](../demo/) for the bad-deploy auto-rollback walkthrough.
