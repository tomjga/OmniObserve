# Build from the REPO ROOT context (docker build -f remediator/Dockerfile .) so the
# incident corpus (incidents/) can be baked in for the RCA copilot to ground on.
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Deps first for layer caching.
COPY remediator/go.mod remediator/go.sum ./
RUN go mod download

# Remediator source (top-level *.go + internal/ packages).
COPY remediator/ ./

# Static binary, version stamped via --build-arg VERSION=...
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.version=${VERSION}" -o remediator .

# Runtime stage
FROM alpine:3.21
WORKDIR /app

# CA certs for outbound HTTPS (Kubernetes API, LLM endpoint, GitHub).
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/remediator /app/
# The incident corpus the RCA copilot retrieves precedent from (CORPUS_DIR=/app/incidents).
COPY incidents /app/incidents

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/healthz || exit 1

USER 65534:65534
CMD ["/app/remediator"]
