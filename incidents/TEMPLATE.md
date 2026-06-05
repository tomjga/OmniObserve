<!--
Filename: YYYY-MM-DD-HHMM-short-slug.md  (24h local time).
The HHMM prefix orders multiple incidents on the same day at a glance — note the
id sequence reflects when an RCA was *written*, not when the incident *occurred*, so
`occurred` below (and the filename time) are the authoritative chronology.
-->
---
id: INC-YYYY-NNNN
title: <one-line description>
date: YYYY-MM-DD
occurred: YYYY-MM-DDTHH:MM:SS-06:00   # ISO-8601 with offset; authoritative ordering key
severity: SEV1 | SEV2 | SEV3 | SEV4
status: investigating | resolved
services: [service-a, service-b]
detection: <how it was detected — alert, SLO breach, user report>
slo_impact: <error budget burned, or "none">
tags: [<retrieval keys: e.g. prometheus, oom, deploy, false-positive>]
remediation: # actions taken — machine-readable for the remediator/retrieval
  - <action 1>
---

## Summary
<2–3 sentences: what happened and the impact.>

## Timeline
- <ts> <event>

## Root cause
<the actual cause, not just the symptom.>

## Resolution
<what stopped the bleeding / fixed it.>

## Lessons / prevention
- <what to change so it doesn't recur; what a future RCA should learn from this.>
