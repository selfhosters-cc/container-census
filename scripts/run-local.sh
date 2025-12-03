#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

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

# Prompt for authentication
read -p "Enable authentication? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Authentication ENABLED (username: qwerty, password: qwerty)"
    AUTH_ENABLED=true
    AUTH_USERNAME=qwerty
    AUTH_PASSWORD=qwerty
    SESSION_SECRET="local-dev-secret-$(date +%s)"
else
    echo "Authentication DISABLED"
    AUTH_ENABLED=false
    AUTH_USERNAME=""
    AUTH_PASSWORD=""
    SESSION_SECRET=""
fi

# Prompt for frontend choice
echo ""
echo "Frontend options:"
echo "  1) Classic (vanilla JS) - web/"
echo "  2) Next.js (React) - web-next/out/"
read -p "Choose frontend [1]: " -n 1 -r
echo

WEB_DIR="$PROJECT_ROOT/web"
if [[ $REPLY == "2" ]]; then
    # Check if Next.js build exists
    if [ -d "$PROJECT_ROOT/web-next/out" ]; then
        WEB_DIR="$PROJECT_ROOT/web-next/out"
        echo -e "${GREEN}Using Next.js frontend${NC}"
    else
        echo -e "${YELLOW}Next.js build not found. Building now...${NC}"
        (cd "$PROJECT_ROOT/web-next" && npm run build)
        WEB_DIR="$PROJECT_ROOT/web-next/out"
        echo -e "${GREEN}Using Next.js frontend${NC}"
    fi
else
    echo -e "${GREEN}Using classic frontend${NC}"
fi

# Run the server with local development settings
echo ""
echo -e "${YELLOW}Starting server on http://localhost:3333${NC}"
echo ""

DOCKER_HOST="${DOCKER_HOST:-unix:///var/run/docker.sock}" \
SERVER_PORT=3333 \
CONFIG_PATH=/opt/docker-compose/census-server/census/config/${CONFIG_FILE} \
AUTH_ENABLED=${AUTH_ENABLED} \
AUTH_USERNAME=${AUTH_USERNAME} \
AUTH_PASSWORD=${AUTH_PASSWORD} \
SESSION_SECRET=${SESSION_SECRET} \
DATABASE_PATH=/opt/docker-compose/census-server/census/server/${DB_FILE} \
TRIVY_CACHE_DIR=/tmp/trivy-cache \
WEB_DIR=${WEB_DIR} \
/tmp/census-server
