# OmniObserve visual control room

Static exercise UI for the remediator approval gate. The main page stays on the exercise
surface; the operational control room opens as an inline card when needed.

## Run

Open `index.html` directly, or serve the folder:

```bash
cd visual
python3 -m http.server 4173
```

Default targets:

- Remediator: `http://localhost:8080`
- Prometheus: `http://localhost:9090`

For a cluster demo:

```bash
kubectl -n monitoring port-forward svc/remediator 8080:8080
kubectl -n monitoring port-forward svc/kps-kube-prometheus-stack-prometheus 9090:9090
```

If `.Values.approval.auth.enabled=true`, paste the approval token into the UI before
approving or denying a request.

## Human approval flow

1. Set `REMEDIATOR_AUTONOMY_MODE=approval`, or set a fault catalog policy to `approval`.
2. Alertmanager posts a firing alert.
3. The remediator records `needs_human`, creates a pending approval, and exports:
   - `remediator_pending_approvals`
   - `remediator_approvals_total{decision="pending"}`
4. The visual lists the pending item.
5. A human chooses `Approve fix`; the remediator executes the bounded flagd action and
   records the normal `remediator_actions_total` and verification/RCA metrics.

The approval store is in-memory in this phase. Metrics and action audit counters remain the
source of reporting truth.
