# Demo: SLO-gated auto-rollback

Shows the Phase 1 payoff end to end: a canary that breaches the error-rate SLO is
**automatically aborted and rolled back** by Argo Rollouts — no human in the loop.

## Prerequisites

- Local Kubernetes cluster (kind/k3d/minikube) with the LGTM stack + kube-prometheus-stack
- Argo Rollouts controller installed ([../argo-rollouts/](../argo-rollouts/))
- `api-service` deployed as a Rollout:
  `helm upgrade --install api-service deploy/api-service -n monitoring --set rollout.enabled=true`
- Prometheus scraping `job="api-service"` (the chart's ServiceMonitor does this)

## Walkthrough

1. **Baseline load** — keep healthy traffic flowing so the SLO metric is populated:

   ```bash
   ./demo/load.sh                 # steady 200s against /kpi/availability
   ```

2. **Trigger a canary** — any pod-template change starts a rollout. Bump the image or
   an annotation:

   ```bash
   kubectl argo rollouts get rollout api-service -n monitoring --watch &
   kubectl patch rollout api-service -n monitoring --type merge \
     -p '{"spec":{"template":{"metadata":{"annotations":{"demo/rev":"bad"}}}}}'
   ```

3. **Inject the regression** — simulate a broken release by driving 5xx through the
   service while the canary is live:

   ```bash
   ./demo/load.sh --errors        # floods /kpi/errors?error_rate=100
   ```

4. **Observe** — within ~a minute the background `AnalysisRun` sees the 5xx ratio cross
   `errorRatioThreshold`, marks the analysis **Failed**, and Argo Rollouts **aborts**
   the canary and shifts all traffic back to the stable ReplicaSet. The watch shows the
   rollout `Degraded → ▮ aborted`, then healthy on the stable revision.

Stop the regression load (`Ctrl-C`) and the next rollout attempt promotes cleanly.

> The app is a KPI simulator, so the "bad release" is simulated via its `/kpi/errors`
> knob. The control loop being demonstrated — SLO query → analysis fail → auto-rollback
> — is exactly what a real regression would trigger.
