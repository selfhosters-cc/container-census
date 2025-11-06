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

    # Detect Docker socket GID and add census user to that group
    # This handles cases where the host's Docker GID differs from build-time DOCKER_GID
    if [ -S /var/run/docker.sock ]; then
        SOCK_GID=$(stat -c '%g' /var/run/docker.sock 2>/dev/null || true)
        if [ -n "$SOCK_GID" ] && [ "$SOCK_GID" != "0" ]; then
            echo "Detected Docker socket GID: $SOCK_GID"
            # Check if group exists, create if not
            if ! getent group "$SOCK_GID" > /dev/null 2>&1; then
                echo "Creating group for GID $SOCK_GID..."
                addgroup -g "$SOCK_GID" "docker_host" 2>/dev/null || true
            fi
            # Add census user to the group
            SOCK_GROUP=$(getent group "$SOCK_GID" | cut -d: -f1)
            if [ -n "$SOCK_GROUP" ]; then
                echo "Adding census user to group $SOCK_GROUP (GID $SOCK_GID)..."
                adduser census "$SOCK_GROUP" 2>/dev/null || true
            fi
        fi
    fi

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
#   AUTH_ENABLED - Enable authentication for UI and API (true/false)
#   AUTH_USERNAME - Username for authentication
#   AUTH_PASSWORD - Password for authentication
#   SCANNER_INTERVAL_SECONDS - Override scan interval
#   TELEMETRY_ENABLED - Override telemetry enabled (true/false)
#   TELEMETRY_INTERVAL_HOURS - Override telemetry interval

database:
  path: ./data/census.db  # Can be overridden by DATABASE_PATH env var

server:
  host: 0.0.0.0  # Can be overridden by SERVER_HOST env var
  port: 8080     # Can be overridden by SERVER_PORT env var
  auth:
    enabled: false  # Set to true to enable authentication (override with AUTH_ENABLED)
    username: ""    # Username for authentication (override with AUTH_USERNAME)
    password: ""    # Password for authentication (override with AUTH_PASSWORD)

scanner:
  interval_seconds: 300  # Scan every 5 minutes (override with SCANNER_INTERVAL_SECONDS)
  timeout_seconds: 30    # Timeout for each scan operation

telemetry:
  enabled: false  # Set to true to enable anonymous telemetry (override with TELEMETRY_ENABLED)
  interval_hours: 168  # Submit telemetry weekly - 7 days (override with TELEMETRY_INTERVAL_HOURS)
  endpoints:
    # Community telemetry endpoint (optional - helps improve container-census)
    - name: community
      url: https://cc-telemetry.selfhosters.cc/api/ingest
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
