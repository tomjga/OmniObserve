# Grafana Alloy

Optional replacement for the retired Grafana Agent Operator manifests.

OmniObserve's primary telemetry path is still application OTLP -> OTel Collector -> LGTM.
Alloy is only needed when the local stack wants Kubernetes pod logs from stdout/stderr
delivered to Loki.

```bash
kubectl apply -f LGTM/alloy/k8s.yaml
kubectl -n monitoring rollout status ds/alloy
```

The Alloy config tails pod logs through the Kubernetes API, labels them with namespace, pod,
container, app, and job, extracts HTTP status codes from the local Go services when present,
and writes to `http://loki-gateway.monitoring.svc.cluster.local/loki/api/v1/push`.
