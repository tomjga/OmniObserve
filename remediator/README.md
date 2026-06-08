# remediator — OmniObserve's control loop

The service that turns alerts into **action** and **explanation**. It receives
**Alertmanager** webhooks when an SLO burns, takes a bounded auto-remediation, and drafts
a grounded RCA — closing the loop from *"we can see the problem"* to *"the system handled
it, and here's why it happened."* See
[`docs/phase-2-auto-remediation.md`](../docs/phase-2-auto-remediation.md) for the design.

## What it does

- `POST /webhook` — parse an Alertmanager payload; log + count alerts
  (`remediator_alerts_received_total`).
- `GET /approvals`, `POST /approvals/:id/approve`, `POST /approvals/:id/deny` —
  human-in-the-loop endpoints for approval-mode remediation. Approval requests are counted
  by `remediator_approvals_total` and currently held in-memory; `remediator_pending_approvals`
  reports the queue depth.
- **Bounded action** — for a firing alert whose `remediation_flag` annotation names a flagd
  flag, set that flag's `defaultVariant` to `off` in the flagd ConfigMap (flagd hot-reloads
  and pushes to consumers — no restarts). Dry-run toggle, per-incident cooldown persisted
  on the ConfigMap annotation `omniobserve.io/remediation-cooldowns`, idempotent,
  least-privilege RBAC scoped to the one ConfigMap. Audited by `remediator_actions_total`
  with explicit outcomes: `healed`, `already_safe`, `cooldown_skipped`, `unsupported`,
  `failed`, and `needs_human`.
- **Catalog-driven policy** — action lookup comes from the mounted fault catalog
  (`REMEDIATION_CATALOG_PATH`), not from arbitrary alert annotations. Alert annotations can
  remain for human readability, but the catalog is authoritative.
- **Webhook auth + stop switch** — optional bearer auth protects `POST /webhook`, and
  `REMEDIATOR_STOP=true` keeps the loop observable while preventing mutation.
- **Progressive autonomy** — `REMEDIATOR_AUTONOMY_MODE` supports `observe`, `suggest`,
  `approval`, `auto`, and `auto-with-verify`. The global mode is a ceiling on the mounted
  catalog policy; `auto-with-verify` waits for recovery evidence before enqueueing the RCA
  so the draft records whether the signal recovered.
- **Approval gate** — in `approval` mode, the loop records `needs_human`, creates a pending
  request, and waits for an authenticated human/tool to approve the exact bounded action.
  Approval executes the same flagd disable path, cooldowns, verification, RCA enqueueing,
  metrics, and logs as autonomous remediation.
- **No-op storm guard** — repeated `already_safe` outcomes for the same flag/incident are
  suppressed and escalated as `needs_human`, counted by `remediator_noop_storms_total`.
- **Post-action verification** — after a real heal, sample Prometheus before/after and record
  whether the signal `improved`, `not_improved`, or lacked enough data via
  `remediator_action_verifications_total`.
- **RCA copilot** — on a real remediation, asynchronously: gather Prometheus evidence,
  retrieve relevant prior incidents from the baked-in corpus (`internal/corpus`), and enqueue
  bounded LLM work (`RCA_WORKERS`, `RCA_QUEUE_DEPTH`) for a structured RCA grounded in that
  material, then publish to the configured sinks (`internal/sink`). Audited by
  `remediator_rca_drafts_total`, `remediator_rca_draft_duration_seconds`, and
  `remediator_rca_queue_total`.
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

## Safety controls

| Control | How it works |
|---|---|
| Scope | The only mutation is setting one named flagd flag's `defaultVariant` to `off`. |
| Persistence | Cooldowns live on the flagd ConfigMap annotation, so restarts do not reset them. |
| Dry-run | `REMEDIATOR_DRY_RUN=true` previews the action without mutating flags or annotations. |
| Stop switch | `REMEDIATOR_STOP=true` blocks mutation and records `needs_human`. |
| Autonomy | `REMEDIATOR_AUTONOMY_MODE` gates whether the loop observes, suggests, waits for approval, acts, or acts only with RCA verification. |
| Webhook auth | `WEBHOOK_BEARER_TOKEN` requires Alertmanager to send `Authorization: Bearer ...`. |
| Approval auth | `APPROVAL_BEARER_TOKEN` protects approval list/approve/deny endpoints. |
| Catalog | `REMEDIATION_CATALOG_PATH` points at the JSON fault/action catalog. |
| No-op storm guard | `REMEDIATOR_NOOP_STORM_THRESHOLD` and `REMEDIATOR_NOOP_STORM_WINDOW_SECONDS` bound repeated already-safe alerts. |
| Verification | `REMEDIATOR_VERIFY_DELAY_SECONDS` controls how long to wait before checking the post-action signal. |
| Concurrency | RCA drafting uses a bounded worker queue; overflow is counted and dropped. |
| RBAC | The ServiceAccount can read/update only the configured flagd ConfigMap. |

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
