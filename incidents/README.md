# Incident RCA compendium

A growing corpus of structured incident root-cause analyses. It is the **institutional
memory** that the Phase 2 RCA copilot retrieves from, and the safety check the
auto-remediation loop consults before acting.

## Why it exists

- **Grounded RCAs.** An LLM analysing an incident cold produces generic output. Given
  similar past incidents as context, it grounds its analysis in *this system's* real
  services, terminology, and proven fixes — higher signal, fewer hallucinations.
- **Safer remediation.** Before auto-remediating, the loop asks "have we seen this
  signature before, and what worked?" — precedent, not guesswork.
- **Turns dormant data into value.** Companies already sit on years of postmortems in
  Jira / ServiceNow / Confluence / PagerDuty. Normalised into this format, that history
  becomes a live assistant (see *Ingesting existing data*).
- **A flywheel.** Every incident the loop handles is written back here, so the corpus —
  and the quality of future RCAs and remediations — compounds over time.

## Format

One Markdown file per incident, named `YYYY-MM-DD-slug.md`, using
[`TEMPLATE.md`](TEMPLATE.md). The **YAML frontmatter is structured** (for filtering and
retrieval); the body is a readable postmortem. Dual-use: humans read it, machines embed it.

## How the LLM uses it (Phase 2)

```
new incident → signature (services + symptoms + metric shape)
            → retrieve top-k similar RCAs (vector search over this corpus)
            → feed as context to the RCA copilot (Claude)
            → draft RCA + suggested remediation, grounded in precedent
            → on resolution, write the new RCA back here
```

## Ingesting existing data

A normaliser maps existing sources into this schema — the `remediation`, `services`,
`tags`, and root-cause fields are what retrieval keys on:

| Source | Maps to |
|--------|---------|
| Jira / ServiceNow incident | id, severity, timeline, services |
| Confluence / Google Docs postmortem | summary, root cause, lessons |
| PagerDuty / Opsgenie | detection, duration, on-call timeline |
| Slack incident channel | timeline, actions taken |

> Start small: the loop seeds the corpus with its own events. Backfill from existing
> sources when connecting to a real org's data.
