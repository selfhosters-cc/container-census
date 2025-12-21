#!/bin/bash
# Build the telemetry collector binary locally
# Output: /tmp/telemetry-collector

set -e

cd "$(dirname "$0")/.."

# Get build timestamp in RFC3339 format
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Building telemetry collector..."
CGO_ENABLED=1 go build \
  -ldflags "-X github.com/selfhosters-cc/container-census/internal/version.BuildTime=${BUILD_TIME}" \
  -o /tmp/telemetry-collector \
  ./cmd/telemetry-collector

echo "Built: /tmp/telemetry-collector"
ls -lh /tmp/telemetry-collector
