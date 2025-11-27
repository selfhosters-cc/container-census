#!/bin/bash
# Build the telemetry collector binary locally
# Output: /tmp/telemetry-collector

set -e

cd "$(dirname "$0")/.."

echo "Building telemetry collector..."
CGO_ENABLED=1 go build -o /tmp/telemetry-collector ./cmd/telemetry-collector

echo "Built: /tmp/telemetry-collector"
