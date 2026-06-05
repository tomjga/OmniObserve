# remediator — OmniObserve's control loop

The service that turns alerts into **action**. It receives **Alertmanager** webhooks when
an SLO burns and (in later steps) takes a bounded, auditable remediation — closing the
loop from *"we can see the problem"* to *"the system handled it."* See
[`docs/phase-2-auto-remediation.md`](../docs/phase-2-auto-remediation.md) for the full design.

## Status: step 1 — observe-only

This first cut **takes no action**. It validates the alert path end-to-end before the
loop is given any power:

- `POST /webhook` — parse an Alertmanager payload, log each alert, and count it
  (`remediator_alerts_received_total{alertname,status}`).
- `GET /healthz` — liveness + build version.
- `GET /metrics` — Prometheus metrics.

It is itself **OpenTelemetry-instrumented** (service `remediator`), so its decisions show
up as traces next to the incidents it reacts to — the platform observes its own control loop.

## Next steps (see the design doc)

2. Wire the SLO alert → Alertmanager → this webhook.
3. First bounded action: **disable the offending feature flag** (flagd kill switch),
   behind a dry-run flag, idempotent per `incident_key`, rate-limited.
4. RCA copilot: pull the incident window's traces/metrics → vendor-agnostic LLM
   ([[vendor-agnostic-llm]]) → grounded RCA → incident file + GitHub issue + Grafana annotation.

## Run locally

```bash
go test -race ./...
go run .            # listens on :8080
# simulate an alert:
curl -XPOST localhost:8080/webhook -H 'content-type: application/json' \
  -d '{"status":"firing","alerts":[{"status":"firing","labels":{"alertname":"HighErrorRate","service":"cart","severity":"critical"},"annotations":{"summary":"cart 5xx burning SLO"}}]}'
```
