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
