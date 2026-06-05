---
id: INC-2026-0005
title: kube-apiserver unreachable (TLS handshake timeout) under CPU starvation
date: 2026-06-04
occurred: 2026-06-04T19:30:00-06:00
severity: SEV2
status: resolved
services: [kube-apiserver, opentelemetry-demo]
detection: kubectl failing with "net/http: TLS handshake timeout" / "context deadline exceeded"; docker socket also down
slo_impact: full local control-plane outage (pre-prod) — no kubectl, no scheduling
tags: [capacity, cpu, control-plane, apiserver, resource-limits, load-generator, single-node]
remediation:
  - Give the node enough CPU (4 -> 8 vCPU) and remove the constant load source
  - On a wedged apiserver, restart the VM then immediately scale the offending workload to 0 before it re-saturates
---

## Summary
Deploying the full OpenTelemetry Demo onto a 4-vCPU local node made the **kube-apiserver
unreachable**: `kubectl` returned `TLS handshake timeout`, then `context deadline
exceeded`. The container runtime's docker socket dropped too. The control plane could not
recover on its own.

## Timeline
- Demo install (~20 services) plus a migration that left old + new pods running
  simultaneously, with the bundled **load generator** driving constant traffic.
- CPU pinned at 100% of 4 cores; apiserver could no longer complete TLS handshakes.
- A `sudo` re-run made it worse — root's kubeconfig couldn't reach the cluster *and* it
  left the user's helm config root-owned (a second, unrelated failure mode).

## Root cause
**CPU starvation of the control plane.** The node had 20 GB RAM (never the constraint)
but only 4 vCPUs. The demo's continuous load generator + a cold-start burst (image pulls,
JVM/LLM startup) + a double-load migration window saturated every core. The apiserver and
etcd are unprotected workloads on this single-node cluster, so they lost the CPU race and
the whole control plane wedged — a chicken-and-egg failure (can't `kubectl` to relieve
the thing that's blocking `kubectl`).

## Resolution
1. Raised the VM to **8 vCPU** (host had 10 cores); the restart also recovered the
   apiserver.
2. Won the race on reboot: scaled the demo to **0** the instant the API answered, let the
   baseline (Tempo + Prometheus) stabilize, then brought the workload back **staged** and
   watched `kubectl top`.
3. Removed the constant load source (`load-generator.enabled: false`) and the heavy local
   `llm` service. Steady-state CPU then sat at ~3%; the crash had been **startup burst**,
   not steady load.

## Lessons / prevention
- On a single-node cluster the control plane competes with workloads for CPU. **Right-size
  the node** and cut constant background load (drop the load generator; inject load
  deliberately instead).
- A wedged apiserver is a deadlock — recovery is *restart, then scale the offender to 0
  before it re-saturates*. Stage workloads back up while watching CPU.
- 20 GB RAM idle while CPU is pinned: diagnose the **actual** bottleneck before assuming
  "needs more memory."
- Never `sudo` kubectl/helm — it uses root's kubeconfig (cluster unreachable) and can
  leave config files root-owned.
