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

For the final portfolio cut, use the 90-second recording package in
[`recording.md`](recording.md).

---

# Demo: autonomous self-heal + RCA (Phase 2)

The Phase 2 payoff in **one command**: a runtime fault (not a bad deploy) is injected,
and the whole loop runs with no human in it — SLO burn → alert → the `remediator`
disables the offending feature flag (heal) → the RCA copilot drafts a grounded analysis.

## Prerequisites

- The Phase 1.5 telemetry stack + OTel Demo deployed (`./bootstrap-telemetry.sh`)
- The `remediator` installed (`./bootstrap.sh`), with flagd repointed to watch its
  ConfigMap so a patch reloads live ([INC-2026-0007](../incidents/))
- For the RCA: an LLM key wired (`rca.llm.*` + the `remediator-rca` Secret). Without it
  the loop still heals — it just runs action-only. See [../remediator/](../remediator/).

## Walkthrough

```bash
./demo/chaos.sh            # product-catalog default: inject -> heal -> RCA
./demo/chaos.sh ad         # run the ad catalog entry
./demo/chaos.sh cart       # run the cart catalog entry
./demo/chaos.sh --status   # current product-catalog flag + recent remediator activity
./demo/chaos.sh --status ad
./demo/chaos.sh --reset cart
```

The script reads `deploy/remediator/files/fault-catalog.json`, flips the selected service's
flag **on**, then polls until the remediator flips it back **off** (the heal), reports the
time-to-heal, and surfaces this run's `remediation` and `rca drafted` log lines. The
existing otel-demo load generator already drives product-catalog traffic; lower-traffic
services may need extra load before their alert fires.

**Validated run:** heal in ~130s (the alert's `for: 1m` + scrape interval + error-ratio
build-up under low load), followed by a ~2.5k-char Gemini RCA grounded in the 7-incident
corpus.

> The RCA is *drafted* regardless, but is only **visible** where a sink is configured —
> a Grafana annotation on the incident window, a GitHub issue, and a committed corpus
> draft. With no sink the script reports `rca drafted (chars=N)` only. Enable the Grafana
> sink (`GRAFANA_TOKEN` in the Secret) to read the full text on the timeline.
