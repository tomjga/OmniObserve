# remediator — OmniObserve's control loop

The service that turns alerts into **action** and **explanation**. It receives
**Alertmanager** webhooks when an SLO burns, takes a bounded auto-remediation, and drafts
a grounded RCA — closing the loop from *"we can see the problem"* to *"the system handled
it, and here's why it happened."* See
[`docs/phase-2-auto-remediation.md`](../docs/phase-2-auto-remediation.md) for the design.

## What it does

- `POST /webhook` — parse an Alertmanager payload; log + count alerts
  (`remediator_alerts_received_total`).
- **Bounded action** — for a firing alert whose `remediation_flag` annotation names a flagd
  flag, set that flag's `defaultVariant` to `off` in the flagd ConfigMap (flagd hot-reloads
  and pushes to consumers — no restarts). Dry-run toggle, per-incident cooldown, idempotent,
  least-privilege RBAC scoped to the one ConfigMap. Audited by `remediator_actions_total`.
- **RCA copilot** — on a real remediation, asynchronously: gather Prometheus evidence,
  retrieve relevant prior incidents from the baked-in corpus (`internal/corpus`), and ask a
  **vendor-agnostic** LLM (`internal/llm`) for a structured RCA grounded in that material,
  then publish to the configured sinks (`internal/sink`). Audited by
  `remediator_rca_drafts_total`.
- `GET /healthz`, `GET /metrics`; OpenTelemetry-traced as service `remediator` — the
  platform observes its own control loop.

## Packages

| Package | Role |
|---|---|
| `internal/llm` | OpenAI-compatible chat client — provider chosen by base URL + model + key |
| `internal/corpus` | Loads `incidents/*.md`, retrieves precedent by tag/keyword overlap (no embeddings) |
| `internal/evidence` | Prometheus instant queries (gRPC + HTTP RED metrics) per service |
| `internal/rca` | The copilot: evidence + precedent + alert → grounded RCA prompt → LLM |
| `internal/sink` | Grafana annotation, GitHub issue, GitHub corpus-draft sinks (best-effort, config-gated) |

## Enabling the RCA copilot (needs an LLM key)

Without an LLM key the loop is **action-only** (the copilot logs `enabled:false`). To turn
it on, set the non-secret config in the chart and provide the secret:

```bash
# 1) non-secret: pick any OpenAI-compatible endpoint + model (vendor-agnostic)
helm upgrade remediator deploy/remediator -n monitoring --reuse-values \
  --set rca.llm.baseURL=https://generativelanguage.googleapis.com/v1beta/openai \
  --set rca.llm.model=gemini-2.0-flash \
  --set rca.github.repo=tomjga/OmniObserve

# 2) secret (gitignored): cp the example, fill keys, apply
cp deploy/remediator/rca-secret.example.yaml deploy/remediator/rca-secret.yaml
#   edit LLM_API_KEY (+ optional GRAFANA_TOKEN / GITHUB_TOKEN), then:
kubectl -n monitoring apply -f deploy/remediator/rca-secret.yaml
```

## Run locally

```bash
go test -race ./...
go run .            # listens on :8080 (observe-only without a cluster)
```
