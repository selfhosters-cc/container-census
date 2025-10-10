# Multi-stage build for Container Census

# Stage 1: Build the Go binary
FROM golang:1.21-alpine AS builder

# Install build dependencies
# Note: sqlite requires specific build tags on Alpine
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /build

# Copy dependency files first (better caching)
COPY go.mod go.sum* ./

# Download dependencies (cached unless go.mod/go.sum changes)
RUN go mod download

# Copy source code
COPY . .

# Tidy if needed (rarely changes cache)
RUN go mod tidy -e

# Build the binary with proper tags for Alpine
RUN CGO_ENABLED=1 GOOS=linux go build -tags "sqlite_omit_load_extension" -o census ./cmd/server

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Build arg for docker group GID (defaults to 999, can be overridden at runtime)
ARG DOCKER_GID=999

# Install ca-certificates for HTTPS and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create docker group with default GID and census user
# Note: The actual GID can be added at runtime using docker-compose group_add
# Delete existing group with same GID if it exists
RUN (getent group ${DOCKER_GID} && delgroup $(getent group ${DOCKER_GID} | cut -d: -f1)) || true && \
    addgroup -g ${DOCKER_GID} docker && \
    addgroup -g 1000 census && \
    adduser -D -u 1000 -G census census && \
    adduser census docker

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/census .

# Copy version file
COPY --from=builder /build/.version ./.version

# Copy web frontend
COPY --from=builder /build/web ./web

# Copy example config
COPY --from=builder /build/config/config.yaml.example ./config/config.yaml.example

# Create data directory with correct permissions
RUN mkdir -p ./data && chown -R census:census /app

# Switch to non-root user
USER census

# Expose HTTP port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# Set environment variables
ENV CONFIG_PATH=/app/config/config.yaml

# Run the application
CMD ["./census"]
