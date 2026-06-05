# Phase 2 — Auto-remediation + RCA copilot (the differentiator)

**The problem.** Phases 0–1.5 made the system *observable* and made *deploys* prove
themselves (Argo Rollouts auto-aborts a bad release). But most incidents aren't bad
deploys — they're **runtime regressions** in already-running software: a dependency
fails, a feature misbehaves, latency creeps. Today those still page a human. Observability
that only *shows* problems leaves the most valuable step — **deciding and acting** — manual.

**What this phase builds.**
1. **A `remediator` service (Go).** Receives **Alertmanager** webhooks when an SLO burns,
   and takes a **safe, bounded, auditable action** to stop the bleeding. Every action is
   idempotent, rate-limited, dry-run-able, and recorded (a metric + an entry in the
   [incident corpus](../incidents/)).
2. **An RCA copilot (vendor-agnostic LLM).** For the incident window it pulls the evidence
   — Tempo traces, Prometheus metrics, (later) Loki logs — and asks an
   **OpenAI-compatible** model (config-only; Gemini/Claude/OpenAI/local — no provider in
   code, see [[vendor-agnostic-llm]]) to draft a **structured RCA**, *grounded in the
   existing incident corpus* (RAG). The draft lands where humans already look.

**The closed loop (demoable in ~90s):**
```
flagd feature flag injects a fault  ──▶  OTel Demo errors / latency climb
        │                                         │
        ▼                                         ▼
   Prometheus SLO burn-rate alert  ──▶  Alertmanager  ──▶  remediator
        │                                                      │
        │                                   ┌──────────────────┴───────────────┐
        ▼                                   ▼                                   ▼
  RCA copilot pulls traces/metrics    bounded action (stop the bleed)    record incident
  → grounded RCA draft                                                   (metric + corpus)
```

**Why it matters.**
- **AIOps / auto-remediation** is the frontier of SRE — moving from *alerting* to
  *self-healing*. This is the portfolio's headline.
- The RCA copilot shows the **real** value of LLMs in ops: not chat, but
  **evidence-grounded** root-cause drafting that gets sharper as the incident corpus grows
  — exactly the pitch when a company "already has scrapeable incident data" ([[rca-compendium]]).
- It spans all three identities ([[career-goal]]): SRE (SLOs, incident response), SWE (a
  real Go control-loop service + LLM integration), and AI-era relevance.

**Safety principles (non-negotiable).**
- **Bounded:** the remediator can only perform a small allowlist of reversible actions.
- **Dry-run first:** ship in observe/annotate mode; enable mutation behind a flag.
- **Idempotent + rate-limited:** never thrash; one action per incident key per cooldown.
- **Auditable:** every decision emits a metric and writes an incident record.

**Build order (each step leaves the repo committable):**
1. ✅ `remediator` skeleton — Alertmanager webhook receiver, `/healthz`, `/metrics`, OTel
   traced, structured logs. *Observe-only:* log + count alerts, take no action yet.
2. ✅ Wire the SLO alert → Alertmanager → remediator (AlertmanagerConfig; matcher
   strategy `None` for cross-namespace routing — [INC-2026-0006](../incidents/)).
3. ✅ The **bounded action**: disable the offending flagd flag (dry-run toggle, cooldown,
   idempotent, least-privilege RBAC). flagd repointed to watch the ConfigMap so a patch
   reloads live — [INC-2026-0007](../incidents/). **Validated hands-off on-cluster:**
   `productCatalogFailure` → SLO alert → remediator → flag off → heal.
4. ⏳ The **RCA copilot**: incident-window evidence pull + vendor-agnostic LLM + corpus RAG.
5. ⏳ Chaos demo: one `flagd` flip drives the whole loop unattended.

> Open decisions are tracked at the top of the relevant step as we reach it.
