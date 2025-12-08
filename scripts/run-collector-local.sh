#!/bin/bash

# Script to build and run telemetry collector locally for testing
# Uses port 8889 to avoid conflicts with production collector on 8081

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Building Telemetry Collector...${NC}"

# Build the collector
cd "$(dirname "$0")/.."
CGO_ENABLED=0 go build -o /tmp/telemetry-collector ./cmd/telemetry-collector

if [ $? -eq 0 ]; then
    echo -e "${GREEN}Telemetry Collector built successfully!${NC}"
    ls -lh /tmp/telemetry-collector
else
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}Starting Telemetry Collector on http://localhost:8889${NC}"
echo ""

# Set environment variables for local testing
export PORT=8889
export DATABASE_URL="postgres://census:census@localhost:5432/telemetry?sslmode=disable"
export COLLECTOR_AUTH_ENABLED=false

# Check if PostgreSQL is available
if ! pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
    echo -e "${RED}WARNING: PostgreSQL does not appear to be running on localhost:5432${NC}"
    echo -e "${YELLOW}The collector will fail to start without a database connection.${NC}"
    echo ""
    echo "To start PostgreSQL, you can use docker-compose:"
    echo "  cd /opt/docker-compose/census-server && docker-compose up -d postgres"
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo "Configuration:"
echo "  PORT: $PORT"
echo "  DATABASE_URL: $DATABASE_URL"
echo "  AUTH_ENABLED: $COLLECTOR_AUTH_ENABLED"
echo ""

# Run the collector
exec /tmp/telemetry-collector
