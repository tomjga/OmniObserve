# If this were built at staff/principal altitude — and how to track service maturity

A reflection note (not a backlog): what would change about OmniObserve if the goal were
org-wide leverage rather than one working loop, plus a simple, honest way to track maturity.

## What changes at staff/principal level

The shift is **altitude**: from *building a self-heal loop* to *making self-healing safe,
adopted, and governed across many teams and services*. Concretely, I'd change:

- **Remediation becomes policy-as-code.** Replace the hard-coded action with a
  `RemediationPolicy` CRD (a controller-runtime operator): an allowlist of reversible actions,
  blast-radius limits, rate limits, approval gates, and **progressive autonomy**
  (observe → suggest → act-with-approval → act). A global "stop button" and an audit trail are
  first-class. The loop we have is one policy instance.
- **Error-budget governance.** SLOs as code (OpenSLO/Sloth), multi-window multi-burn-rate
  alerts, and an **error-budget policy** that both gates deploys and selects the remediation
  tier. Reliability becomes a budget you spend, not a vibe.
- **Confidence-scored, multi-signal RCA.** Fuse metrics + traces + logs + change/deploy events
  into an RCA with a **confidence score**; auto-act only above a threshold, human-in-the-loop
  below. Close the loop: remediation outcomes label the corpus so retrieval gets sharper.
- **Safety & blast radius as design, not hope.** Canary/dry-run remediations in prod, circuit
  breakers, per-radius rate limits, and reversibility guarantees on every action.
- **Supply chain & provenance as table stakes.** SBOM (syft), signing (cosign), Trivy in CI,
  and an **admission policy that only runs signed, attested images.**
- **Cost & reliability as quantified tradeoffs.** Attribute $ to incidents and to the platform;
  report MTTR, toil-hours saved, and cost-avoided. This is how the work gets funded.
- **Observability as a product.** Paved-road instrumentation libraries, golden dashboards and
  SLO templates generated from a **service catalog** — leverage, not per-service one-offs.
- **Org practices.** RFC/design-review culture, blameless incident reviews that feed the
  corpus, on-call/runbook standards, and a maturity model adopted across services.

And the behavioral shift underneath all of it: optimize for **others' leverage** — write the
design docs, set direction and guardrails, measure outcomes (DORA + SLO + maturity), mentor.

## Tracking service maturity — simple yet elegant

The version shipped in the Dashboard (`Progress OS ▸ Observability ▸ Service maturity`,
`src/data/maturity.ts` + `MaturityScorecard.tsx`):

- **Few dimensions** — Observability, Reliability/SLOs, Delivery, Security/Supply-chain,
  Operability, Auto-remediation.
- **Each scored 1–5** against concrete criteria, with a one-line *Now* and the single *Next*
  step that raises the level. Overall = the average.

Why this shape works:
- **Few levels** avoid false precision; a 1–5 you can defend beats a 100-point rubric you can't.
- **One "Next" per dimension** makes the scorecard a *roadmap*, not a report card — it's always
  obvious what to do.
- **Reads at a glance** — a row of segment meters + Now/Next, no training required.

The elegant endgame: make it **data-driven, then auto-derived**. Start hand-scored (where we
are). Then compute levels from signals you already collect — Observability from "are
traces+metrics+logs present?", Reliability from SLO compliance / error-budget burn, Delivery
from which CI gates exist (tests/scan/sign), Security from CodeQL/Dependabot/SBOM presence. A
per-service **catalog** view of these scorecards is the org-wide version of the same idea.
