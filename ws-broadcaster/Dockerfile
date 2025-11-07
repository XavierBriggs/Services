# ==============================================================================
# Multi-stage Dockerfile for WebSocket Broadcaster Service
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Builder
# ------------------------------------------------------------------------------
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod verify

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a \
    -o ws-broadcaster \
    ./cmd/ws-broadcaster

# ------------------------------------------------------------------------------
# Stage 2: Runtime
# ------------------------------------------------------------------------------
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 wsbroadcaster && \
    adduser -D -u 1000 -G wsbroadcaster wsbroadcaster

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/ws-broadcaster .

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Set ownership
RUN chown -R wsbroadcaster:wsbroadcaster /app

# Switch to non-root user
USER wsbroadcaster

# Health check (HTTP endpoint)
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Expose HTTP/WebSocket port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/app/ws-broadcaster"]

