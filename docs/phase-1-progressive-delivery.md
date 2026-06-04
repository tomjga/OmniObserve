# Phase 1 — Progressive delivery

**The problem.** Most outages are self-inflicted — they arrive in a deploy. Shipping a
new version to 100% of users at once means every bug is a full-blown outage.

**What this phase builds.**
- The app deploys as an Argo Rollouts **canary** (25% → 50% → 100%, with pauses).
- An **AnalysisTemplate** continuously queries the error-rate SLO *during* the rollout.
- A version that breaches the SLO is **automatically aborted and rolled back**.
- A healthy version **promotes on its own** — no human in the loop.

**Why it matters.**
- The **SLO gate decides** whether a release ships — not a person watching dashboards,
  not a fixed timer.
- **Blast radius** shrinks: a bad version only ever reaches a fraction of traffic.
- **MTTR** drops: rollback is automatic and immediate, not a 2 a.m. page.
- This is *error-budget-driven delivery* — turning *"hope it's fine"* into *"prove it's
  fine before exposing users."*

**A real bug we caught.** The first healthy canary *failed* — with zero errors, the 5xx
query matched no series and returned **empty**, which the gate read as a failure. The fix
(`or vector(0)`: treat no-data as `0`) is a classic SLO-gating footgun, and exactly the
kind of thing this project exists to surface and document.

**See it:** the [demo](../demo/) — a bad canary rolls back, a good one promotes.
