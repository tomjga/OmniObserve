# 90-second portfolio demo

This is the recording package for the final OmniObserve walkthrough. It is intentionally
short: the viewer should see the platform move from detection to autonomous action, then
understand the staff-level design behind it.

## Prerequisites

- `./bootstrap.sh` completed.
- `./bootstrap-telemetry.sh` completed.
- `worker-service` is running so `api-service` has steady traffic.
- Optional RCA visibility: `remediator-rca` Secret has `LLM_API_KEY`, `GRAFANA_TOKEN`, and
  `GITHUB_TOKEN`; without it the demo still heals but the RCA draft is log-only or disabled.
- Grafana reachable:

  ```bash
  kubectl -n monitoring port-forward svc/kps-grafana 3000:80
  ```

## Recording Timeline

| Time | Screen | Action | What the viewer learns |
|---|---|---|---|
| 0-10s | README architecture diagram | Open the repo at the Phase table and architecture diagram. | This is a full local reliability platform, not one script. |
| 10-25s | Argo Rollouts terminal | Show `kubectl argo rollouts get rollout api-service -n monitoring`. | Deploys are SLO-gated and rollback-capable. |
| 25-40s | Grafana dashboard | Show API request rate, error ratio, remediator action outcomes, and worker traffic. | The platform observes itself and the workload. |
| 40-60s | Terminal | Run `./demo/chaos.sh product-catalog`. | Runtime fault injection enters through the catalog. |
| 60-75s | Terminal + Grafana | Show flag healing and `remediator_actions_total{outcome="healed"}`. | The control loop acted safely and reversibly. |
| 75-90s | GitHub/Grafana/terminal RCA | Show the RCA draft destination, or the `rca drafted` log line if sinks are disabled. | The incident is explained with evidence and confidence. |

## Commands

```bash
kubectl argo rollouts get rollout api-service -n monitoring
kubectl -n monitoring get deploy worker-service remediator
kubectl -n monitoring exec deploy/worker-service -- wget -qO- http://127.0.0.1:8080/metrics \
  | grep 'worker_requests_total'
./demo/chaos.sh product-catalog
kubectl -n monitoring logs deploy/remediator --tail=80 \
  | grep -E 'remediation|post-action verification|rca drafted'
```

## UI Verification Checklist

- Grafana has the `OmniObserve Overview` dashboard loaded.
- Dashboard shows:
  - API request/error/latency panels.
  - Worker traffic panel or `worker_requests_total` in Explore.
  - Remediator action outcome panel with `healed`, `already_safe`, `needs_human`, and
    verification labels available.
  - RCA queue/draft latency panels.
  - Post-action verification panel.
- README task list shows P0, P1, and P2 complete except future production-hardening debt.
- If Alloy is installed, Grafana Explore can query pod logs with `{app="api-service"}` or
  `{app="remediator"}`.

## Success Criteria

- Bad deploy rollback demo can be shown from Argo Rollouts.
- Runtime fault heal demo can be shown from `demo/chaos.sh`.
- RCA draft is visible in a configured sink or its draft log is visible from remediator logs.
- Business impact and maturity scorecard are visible in the README for interview narration.
