# ==============================================================================
# Multi-stage Dockerfile for Normalizer Service
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
    -o normalizer \
    ./cmd/normalizer

# ------------------------------------------------------------------------------
# Stage 2: Runtime
# ------------------------------------------------------------------------------
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 normalizer && \
    adduser -D -u 1000 -G normalizer normalizer

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/normalizer .

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Set ownership
RUN chown -R normalizer:normalizer /app

# Switch to non-root user
USER normalizer

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD pgrep normalizer || exit 1

# Expose metrics port (if added later)
# EXPOSE 9091

# Run the binary
ENTRYPOINT ["/app/normalizer"]

