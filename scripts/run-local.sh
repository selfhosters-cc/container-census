#!/bin/bash

# Create Trivy cache directory in /tmp for local development
mkdir -p /tmp/trivy-cache

# Prompt user for test mode
read -p "Run in test mode? (y/N): " -n 1 -r
echo    # move to a new line
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Running in TEST mode..."
    CONFIG_FILE="config-test.yaml"
    DB_FILE="census-test.db"

    # Ask if user wants to reset the test database
    read -p "Reset test database? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Resetting test database..."
        rm -f /opt/docker-compose/census-server/census/config/config-test.yaml /opt/docker-compose/census-server/census/server/census-test.db
    else
        echo "Keeping existing test database..."
    fi
else
    echo "Running in NORMAL mode..."
    CONFIG_FILE="config.yaml"
    DB_FILE="census.db"
fi

# Run the server with local development settings
DOCKER_HOST="${DOCKER_HOST:-unix:///var/run/docker.sock}" \
SERVER_PORT=3000 \
CONFIG_PATH=/opt/docker-compose/census-server/census/config/${CONFIG_FILE} \
AUTH_ENABLED=false \
DATABASE_PATH=/opt/docker-compose/census-server/census/server/${DB_FILE} \
TRIVY_CACHE_DIR=/tmp/trivy-cache \
/tmp/census-server