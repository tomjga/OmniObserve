# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app

# 1. Init module (if absent)
RUN test -f go.mod || go mod init go-kpi-monitor

# 2. Copy deps and download
COPY go.mod go.sum ./
RUN go mod download

# 3. Copy source and docs folder
COPY main.go ./
COPY docs ./docs

# 4. Re-generate swagger if you want to keep it dynamic:
 RUN go install github.com/swaggo/swag/cmd/swag@latest \
 && swag init --generalInfo main.go --output ./docs

# 5. Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o go-kpi-monitor .

# Runtime stage
FROM alpine:3.18
WORKDIR /app

# CA certs for HTTPS calls
RUN apk add --no-cache ca-certificates

# Copy the binary and docs (so swagger.json lives alongside)
COPY --from=builder /app/go-kpi-monitor /app/
COPY --from=builder /app/docs /app/docs

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/healthz || exit 1

USER 65534:65534
CMD ["/app/go-kpi-monitor"]
