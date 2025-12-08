#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse command-line flags
TEST_MODE=false
RESET_DB=false
AUTH_ENABLED=false
FRONTEND="classic"
INTERACTIVE=true

while [[ $# -gt 0 ]]; do
    case $1 in
        --test)
            TEST_MODE=true
            shift
            ;;
        --reset-db)
            RESET_DB=true
            shift
            ;;
        --auth)
            AUTH_ENABLED=true
            shift
            ;;
        --nextjs)
            FRONTEND="nextjs"
            shift
            ;;
        --no-interactive|-y)
            INTERACTIVE=false
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --test              Run in test mode (uses config-test.yaml and census-test.db)"
            echo "  --reset-db          Reset the test database (only valid with --test)"
            echo "  --auth              Enable authentication (username: qwerty, password: qwerty)"
            echo "  --nextjs            Use Next.js frontend instead of classic"
            echo "  --no-interactive,-y Run without prompts (use default/flag settings)"
            echo "  --help,-h           Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                           # Interactive mode (prompts for all options)"
            echo "  $0 --no-interactive          # Non-interactive with defaults (normal mode, no auth, classic UI)"
            echo "  $0 -y --nextjs               # Non-interactive with Next.js UI"
            echo "  $0 --test --auth --nextjs -y # Test mode, auth enabled, Next.js UI, no prompts"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Create Trivy cache directory in /tmp for local development
mkdir -p /tmp/trivy-cache

# Test mode configuration
if [ "$INTERACTIVE" = true ]; then
    read -p "Run in test mode? (y/N): " -n 1 -r
    echo    # move to a new line
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        TEST_MODE=true
    fi
fi

if [ "$TEST_MODE" = true ]; then
    echo "Running in TEST mode..."
    CONFIG_FILE="config-test.yaml"
    DB_FILE="census-test.db"

    if [ "$INTERACTIVE" = true ]; then
        read -p "Reset test database? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            RESET_DB=true
        fi
    fi

    if [ "$RESET_DB" = true ]; then
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

# Authentication configuration
if [ "$INTERACTIVE" = true ]; then
    read -p "Enable authentication? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        AUTH_ENABLED=true
    fi
fi

if [ "$AUTH_ENABLED" = true ]; then
    echo "Authentication ENABLED (username: qwerty, password: qwerty)"
    AUTH_USERNAME=qwerty
    AUTH_PASSWORD=qwerty
    SESSION_SECRET="local-dev-secret-$(date +%s)"
else
    echo "Authentication DISABLED"
    AUTH_USERNAME=""
    AUTH_PASSWORD=""
    SESSION_SECRET=""
fi

# Frontend configuration
if [ "$INTERACTIVE" = true ]; then
    echo ""
    echo "Frontend options:"
    echo "  1) Classic (vanilla JS) - web/"
    echo "  2) Next.js (React) - web-next/out/"
    read -p "Choose frontend [1]: " -n 1 -r
    echo

    if [[ $REPLY == "2" ]]; then
        FRONTEND="nextjs"
    fi
fi

WEB_DIR="$PROJECT_ROOT/web"
if [ "$FRONTEND" = "nextjs" ]; then
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
TELEMETRY_ENDPOINT_URL=http://100.91.119.113:8081/api/ingest \
/tmp/census-server
