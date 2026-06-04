# Phase 0 — Foundations

**The problem.** You can't improve reliability you can't measure — and a pile of tools
isn't a platform. Most "monitoring projects" stop at installing Grafana.

**What this phase builds.**
- A Go service instrumented with **OpenTelemetry** — vendor-neutral traces, metrics, logs.
- An **OTel Collector** that fans telemetry out to the backends, so a backend can be
  swapped without touching application code.
- **SLOs as code** (Sloth) — the definition of "healthy" lives in version control.
- **CI that gates quality** — tests, linting, dependency + secret scanning on every change.
- A **signed, attested** container image — SBOM + provenance + cosign signature.
- A **Helm chart** so the whole thing deploys repeatably.

**Why it matters.**
- **Vendor-neutral telemetry** avoids lock-in — the biggest long-term cost trap in observability.
- **SLOs as code** makes reliability reviewable and versioned, like any other code.
- **Supply-chain signing** answers *"can you prove what's running came from your pipeline?"* —
  increasingly a compliance and security requirement.
- Together these are the difference between *using* observability tools and *operating a platform*.

**See it:** [`bootstrap.sh`](../bootstrap.sh), [`slo/`](../slo/), [`collector/`](../collector/),
[`.github/workflows/`](../.github/workflows/).
