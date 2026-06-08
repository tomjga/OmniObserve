# Build from the repo root context:
#   docker build -f services/worker-service/Dockerfile .
FROM golang:1.25-alpine AS builder
WORKDIR /app/services/worker-service

COPY services/shared/telemetry /app/services/shared/telemetry
COPY services/worker-service/go.mod services/worker-service/go.sum* ./
RUN go mod download

COPY services/worker-service ./
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.version=${VERSION}" -o worker-service .

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/services/worker-service/worker-service /app/

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/healthz || exit 1

USER 65534:65534
CMD ["/app/worker-service"]
