# OmniObserve — project journey

OmniObserve is a **self-healing observability platform**: it detects reliability
regressions from telemetry and remediates them automatically, with an LLM assist for
root-cause analysis.

This folder documents the project **phase by phase** — not just *what* was built, but
*why each piece matters* for running reliable systems.

| Phase | Theme | Status |
|------|-------|--------|
| [0 — Foundations](phase-0-foundations.md) | A trustworthy base you can observe and ship safely | ✅ done |
| [1 — Progressive delivery](phase-1-progressive-delivery.md) | Releases that prove themselves before reaching users | ✅ validated |
| 2 — Auto-remediation + RCA copilot | Telemetry that triggers action, not just alerts; RCAs grounded in an [incident corpus](../incidents/) | ⏳ next |

**The thesis:** observability is only valuable if it drives **decisions and actions**.
Each phase moves one step from *"we can see problems"* toward *"the system handles them."*
