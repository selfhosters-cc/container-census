#!/bin/sh
set -e

# Ensure data directory exists with correct permissions
# This handles both fresh installs and mounted volumes
if [ "$(id -u)" = "0" ]; then
    echo "Setting up data directory..."

    # Create directory if it doesn't exist
    mkdir -p /app/data
    mkdir -p /app/config

    # Always set correct ownership for mounted volumes
    # This is idempotent - safe to run even if already correct
    chown -R census:census /app/data

    # Create default config.yaml if it doesn't exist
    if [ ! -f /app/config/config.yaml ]; then
        echo "Creating default config.yaml..."
        cat > /app/config/config.yaml <<'EOF'
# Container Census Configuration
#
# Environment variables can override these settings:
#   DATABASE_PATH - Override database path
#   SERVER_HOST - Override server host
#   SERVER_PORT - Override server port
#   SCANNER_INTERVAL_SECONDS - Override scan interval
#   TELEMETRY_ENABLED - Override telemetry enabled (true/false)
#   TELEMETRY_INTERVAL_HOURS - Override telemetry interval

database:
  path: ./data/census.db  # Can be overridden by DATABASE_PATH env var

server:
  host: 0.0.0.0  # Can be overridden by SERVER_HOST env var
  port: 8080     # Can be overridden by SERVER_PORT env var

scanner:
  interval_seconds: 300  # Scan every 5 minutes (override with SCANNER_INTERVAL_SECONDS)
  timeout_seconds: 30    # Timeout for each scan operation

telemetry:
  enabled: false  # Set to true to enable anonymous telemetry (override with TELEMETRY_ENABLED)
  interval_hours: 168  # Submit telemetry weekly - 7 days (override with TELEMETRY_INTERVAL_HOURS)
  endpoints:
    # Community telemetry endpoint (optional - helps improve container-census)
    - name: community
      url: http://cc-telemetry.selfhosters.cc:9876/api/ingest
      enabled: false  # Set to true to participate
      api_key: ""  # No authentication required for community endpoint

hosts:
  # Local Docker daemon via Unix socket
  - name: local
    address: unix:///var/run/docker.sock
    description: Local Docker daemon
EOF
        chown census:census /app/config/config.yaml
        echo "Default config.yaml created"
    fi

    echo "Starting as census user..."
    exec su-exec census "$@"
else
    # Already running as census user, just create dirs if needed
    mkdir -p /app/data
    mkdir -p /app/config
    exec "$@"
fi
