# Build from the repo root context:
#   docker build -f services/api-service/Dockerfile .
FROM golang:1.25-alpine AS builder
WORKDIR /app/services/api-service

COPY services/shared/telemetry /app/services/shared/telemetry
COPY services/api-service/go.mod services/api-service/go.sum ./
RUN go mod download

COPY services/api-service ./

RUN go install github.com/swaggo/swag/cmd/swag@latest \
 && swag init --generalInfo main.go --output ./docs

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.version=${VERSION}" -o go-kpi-monitor .

FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/services/api-service/go-kpi-monitor /app/
COPY --from=builder /app/services/api-service/docs /app/docs

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/healthz || exit 1

USER 65534:65534
CMD ["/app/go-kpi-monitor"]
