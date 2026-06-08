# worker-service

Small synthetic workload generator for the local OmniObserve stack. It repeatedly calls
`api-service` so SLOs, traces, dashboards, canary analysis, and remediation have steady
traffic without a manual curl loop.

Configuration:

| Env var | Default |
|---|---|
| `WORKER_TARGET_URL` | `http://api-service:8080/benchmark?max_delay=25` |
| `WORKER_CONCURRENCY` | `2` |
| `WORKER_INTERVAL_MS` | `500` |
| `WORKER_LISTEN_ADDR` | `:8080` |

Endpoints:

- `GET /healthz`
- `GET /metrics`
