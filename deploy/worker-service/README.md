# worker-service Helm chart

Deploys the synthetic workload generator that keeps `api-service` warm for local SLO,
trace, and remediation demos.

```bash
helm upgrade --install worker-service deploy/worker-service -n monitoring
```

Key values: `worker.targetURL`, `worker.concurrency`, `worker.intervalMs`, image settings,
and `otel.endpoint`.
