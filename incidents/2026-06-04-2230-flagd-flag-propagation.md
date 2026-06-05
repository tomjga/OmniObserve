---
id: INC-2026-0007
title: Feature-flag remediation didn't reach services — flagd served a one-time-seeded copy
date: 2026-06-04
occurred: 2026-06-04T22:30:00-06:00
severity: SEV3
status: resolved
services: [flagd, product-catalog, remediator]
detection: Remediator patched the flagd ConfigMap to disable a fault, but services kept failing; flag changes had no effect
slo_impact: none (caught building the remediation action) — but a remediation that silently does nothing is worse than none
tags: [flagd, feature-flags, configmap, openfeature, sync-stream, propagation, kubernetes]
remediation:
  - Mount the flagd ConfigMap directly where flagd watches, so edits hot-reload and push to consumers live — no restarts
---

## Summary
The remediator's action is to disable a flagd feature flag by patching the flagd
ConfigMap. But patching the ConfigMap had **no effect** — the fault kept firing. Even
manually flipping the flag and restarting flagd didn't reliably reach the consuming
services. A remediation that runs successfully yet changes nothing is a dangerous
false sense of safety.

## Timeline
- Injected a fault flag, drove load — services errored as expected (eventually).
- Remediator patched the ConfigMap flag to "off" — services kept erroring.
- flagd's OFREP endpoint *did* report the served value, but consumers ignored changes
  until the consumer pods were restarted.

## Root cause
Two layers of indirection, both restart-sensitive:
1. **flagd doesn't read the ConfigMap.** The demo's flagd watches a **writable emptyDir
   copy** that an **init container seeds once** from the ConfigMap (`cp /config-ro →
   /config-rw`) at pod start. So ConfigMap edits never reach flagd unless flagd restarts
   and re-seeds.
2. **Consumers cache the sync stream.** Services hold an OpenFeature gRPC sync stream to
   `flagd:8013`. Restarting flagd *breaks* those streams, and on reconnect the consumers
   did not reliably pick up the new value — only restarting the *consumer* did.

Net effect: patch ConfigMap → nothing; restart flagd → consumers stale; the only thing
that worked was a cascade of restarts, which is brittle and wrong for a control loop.

## Resolution
Mount the **ConfigMap directly** at the path flagd watches (drop the writable-copy
indirection). Now a ConfigMap edit propagates via kubelet (~60s) to flagd's file, flagd
**hot-reloads in place**, and pushes the delta to consumers over their **already-open
streams** — no flagd restart, no consumer restart. Verified: flipping the flag in the
ConfigMap healed an erroring service in ~60–75s with zero restarts. The remediator's
action is therefore a single ConfigMap patch (no restart logic — which would have *broken*
the sync streams).

## Lessons / prevention
- "The config store was updated" ≠ "the running system changed." Verify the change at the
  **consumer**, not at the source of truth.
- A remediation must converge **without** disruptive side effects. Restarting the thing
  you're trying to fix (or its dependency) is a smell — prefer mechanisms that push
  changes over live connections.
- Understand the *propagation path* of any config you mutate (seed-once copies, caches,
  watch vs. poll). An init-time copy silently turns "live config" into "boot config."
- A future RCA tagged `feature-flags` + `propagation` should check the **watch source**
  and **consumer refresh mechanism** before assuming a write took effect.
