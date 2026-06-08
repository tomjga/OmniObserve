# Backlog

Captured ideas to tackle later — not yet implemented.

## 1. Business / monetary impact (translate reliability into dollars)

For a production application, document the **money** impact of each capability — what a
hiring manager or exec actually cares about. Planned as a `docs/business-impact.md` plus
a short "Why it matters (in dollars)" note per phase.

Concrete angles to cover:
- **Cost of downtime** — $/min for the app; progressive delivery + auto-rollback cut
  outage minutes → $ saved.
- **MTTR reduction** — automatic rollback + RCA copilot save expensive senior-engineer
  hours per incident and reduce revenue lost during outages.
- **Observability spend** — OTel + tail sampling + retention/cardinality tuning cut
  ingest bills (real lever: Splunk→New Relic style 30% reductions).
- **Blast-radius math** — error-budget-driven delivery avoids the cost of a *full* bad
  deploy by exposing only a fraction of traffic.
- **A simple calculator** — inputs (req/s, $/downtime-min, deploy frequency, % bad
  deploys) → estimated $ avoided. Makes the value concrete and defensible.

## 2. Leave room for MCP (Model Context Protocol)

Future-proof the Phase 2 LLM layer so tools/data are exposed via **MCP** rather than
hard-wired into the copilot.

Concrete angles:
- Expose the **incident corpus** (`incidents/`) and telemetry (**Prometheus / Loki /
  Tempo / Kubernetes**) as **MCP servers**, so the RCA copilot — and any MCP client
  (Claude Desktop/Code) — can query them as standardized tools.
- The **remediator / RCA copilot** acts as an **MCP client** consuming those servers.
- Design Phase 2 retrieval + tool access behind an interface an MCP server can wrap
  later, so we don't paint ourselves into a bespoke-integration corner.

## 3. Per-service fault injection (generalize beyond product-catalog)

Today the self-heal demo and the remediator are hard-wired to a single fault:
`productCatalogFailure` on `product-catalog` (one flagd flag, one PrometheusRule, one
bounded action). The OTel demo ships flagd flags for many services — `adServiceFailure`
/ `adServiceHighCpu`, `cartServiceFailure`, `paymentServiceFailure` /
`paymentServiceUnreachable`, `recommendationServiceCacheFailure`, `kafkaQueueProblems`,
`imageSlowLoad`, etc. We should be able to inject a fault in **each** service and drive
the same detect → alert → remediate → RCA loop.

Concrete angles:
- **Fault catalog** — a small declarative map: `service → { flag, PrometheusRule,
  SLO query, bounded remediation action }`. Both the demo bridge and the remediator
  read from it instead of hardcoding `productCatalogFailure`.
- **Per-service SLO rules** — generate a PrometheusRule per service (error-rate /
  latency) so each fault has a real alert that fires.
- **Remediator generalization** — make the bounded action parameterized by the catalog
  entry (disable that service's flag), keeping the cooldown/dry-run/reversibility guards.
- **Demo UX** — a service picker in the live "Run live" widget so you can choose which
  service to break; the error-ratio chart + pod table + RCA follow the selected service.
- **Stretch** — multiple/cascading faults to show blast-radius and dependency-aware RCA
  (ties to [[rca-compendium]] and the Phase 2 RCA copilot).

**Why it matters:** proves the loop is a *general* control system, not a one-off scripted
demo — much stronger interview signal.

## 4. Approval gate hardening (persistence + chat)

The remediator now supports `approval` mode with `GET /approvals` and explicit
Approve/Deny actions from the visual control room. The next hardening step is to make that
gate durable and reachable from the tools an on-call engineer already watches: chat with
**Approve / Deny** buttons. Use a **Slack incoming webhook**, or a free messaging tool
(**Telegram bot**, **Discord webhook**) so it costs nothing.

Concrete angles:
- **Modes**: `auto` (act immediately) vs `approval` (notify + wait).
  Configurable per-policy (ties to the staff-level `RemediationPolicy` /
  [[staff-principal-and-maturity]] progressive-autonomy idea) — e.g. low-blast-radius
  actions auto, higher-risk ones require approval.
- **Persistence**: move pending approvals out of in-memory storage so remediator restarts do
  not lose human decisions waiting in the queue.
- **Notification**: post the incident (alert, evidence, the action the remediator *would*
  take, and the AI's proposed remediation) to the webhook with approve/deny actions or a
  reply command.
- **Wait + timeout**: hold the action for a bounded window; on approve → act, on deny → skip
  and record, on timeout → fall back to the policy default (skip or act).
- **Audit**: record who approved/denied as a metric label + in the RCA, so the decision is
  on the record.

**Why it matters:** shows judgement about *autonomy levels* — auto-remediation isn't all-or-
nothing; you can gate risky actions behind a human while keeping safe ones automatic.
