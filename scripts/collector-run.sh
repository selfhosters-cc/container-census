#!/bin/bash
# Run the telemetry collector locally
# Uses same configuration as /opt/docker-compose/telemetry setup

set -e

BINARY="/tmp/telemetry-collector"

if [ ! -f "$BINARY" ]; then
    echo "Binary not found. Building first..."
    "$(dirname "$0")/collector-build.sh"
fi

# Match production telemetry setup from /opt/docker-compose/telemetry
export DATABASE_URL="${DATABASE_URL:-postgres://postgres:telemetry-postgres-password@localhost:5444/telemetry?sslmode=disable}"
export PORT="${PORT:-3334}"
export TZ="${TZ:-UTC}"
export COLLECTOR_AUTH_ENABLED="${COLLECTOR_AUTH_ENABLED:-false}"
export COLLECTOR_AUTH_USERNAME="${COLLECTOR_AUTH_USERNAME:-collector_user}"
export COLLECTOR_AUTH_PASSWORD="${COLLECTOR_AUTH_PASSWORD:-collector_secure_password}"
export STATS_MIN_INSTALLATIONS="${STATS_MIN_INSTALLATIONS:-1}"  # Lower threshold for local testing

echo "Starting telemetry collector on port $PORT..."
echo "Database: $DATABASE_URL"
exec "$BINARY"
