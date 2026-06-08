# api-service

OTel-instrumented Go/Gin KPI API used by OmniObserve for SLO and progressive-delivery
demos. It exposes health, metrics, Swagger docs, and configurable endpoints for availability,
latency, error-rate, and benchmark traffic.

Run locally:

```bash
go test -race ./...
go run .
```

Build from the repo root so the shared telemetry module is available:

```bash
docker build -f services/api-service/Dockerfile -t omniobserve-api-service:local .
```

Deploy with the chart in `deploy/api-service`.
