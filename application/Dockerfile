# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Initialize Go module if go.mod doesn't exist
RUN test -f go.mod || go mod init kpi-monitor

# Copy dependency files first to leverage Docker cache
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o kpi-monitor .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/kpi-monitor /app/

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the application as non-root user
USER 65534:65534
CMD ["/app/kpi-monitor"]