# Kootenai Platform API Server
# Multi-stage build for minimal image size

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Build args for version embedding
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Copy source code
COPY . .

# Build the binary with version info
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s \
      -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME} \
      -X github.com/toddbartholow/kootenai/api/internal/version.Version=${VERSION} \
      -X github.com/toddbartholow/kootenai/api/internal/version.Commit=${COMMIT} \
      -X github.com/toddbartholow/kootenai/api/internal/version.BuildTime=${BUILD_TIME}" \
    -o labctl \
    ./cmd/labctl

# Runtime stage
FROM alpine:3.21

# Patch OS-level vulnerabilities
RUN apk upgrade --no-cache

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/labctl .

# Create templates directory (will be mounted in compose)
RUN mkdir -p ./templates

# Set ownership
RUN chown -R appuser:appuser /app

USER appuser

# Expose API port
EXPOSE 8080

# Health check (use -O- instead of --spider to send GET request)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O- http://localhost:8080/health || exit 1

# Run the server
ENTRYPOINT ["./labctl"]
CMD ["serve"]
