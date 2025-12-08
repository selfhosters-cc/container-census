#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if we should build the Next.js frontend
BUILD_FRONTEND=false
if [[ "$1" == "--with-frontend" || "$1" == "-f" ]]; then
    BUILD_FRONTEND=true
fi

# Build Next.js frontend if requested
if [[ "$BUILD_FRONTEND" == "true" ]]; then
    echo -e "${YELLOW}Building Next.js frontend...${NC}"

    if [ ! -d "$PROJECT_ROOT/web-next/node_modules" ]; then
        echo "Installing npm dependencies..."
        (cd "$PROJECT_ROOT/web-next" && npm install)
    fi

    (cd "$PROJECT_ROOT/web-next" && npm run build)

    # Copy the static export to a temporary location for serving
    echo -e "${GREEN}Frontend built successfully!${NC}"
    echo "Static files available in: $PROJECT_ROOT/web-next/out/"
fi

# Build Graph Plugin Frontend
GRAPH_PLUGIN_DIR="$PROJECT_ROOT/internal/plugins/builtin/graph/frontend"
if [ -d "$GRAPH_PLUGIN_DIR/src" ]; then
    echo -e "${YELLOW}Building Graph Plugin frontend...${NC}"

    # Check if node_modules exists, if not install dependencies
    if [ ! -d "$GRAPH_PLUGIN_DIR/node_modules" ]; then
        echo "Installing Graph Plugin npm dependencies..."
        (cd "$GRAPH_PLUGIN_DIR" && npm install)
    fi

    # Build the webpack bundle
    (cd "$GRAPH_PLUGIN_DIR" && npm run build)

    echo -e "${GREEN}Graph Plugin frontend built successfully!${NC}"
fi

# Build Security Plugin Frontend
SECURITY_PLUGIN_DIR="$PROJECT_ROOT/internal/plugins/builtin/security/frontend"
if [ -d "$SECURITY_PLUGIN_DIR/src" ]; then
    echo -e "${YELLOW}Building Security Plugin frontend...${NC}"

    # Check if node_modules exists, if not install dependencies
    if [ ! -d "$SECURITY_PLUGIN_DIR/node_modules" ]; then
        echo "Installing Security Plugin npm dependencies..."
        (cd "$SECURITY_PLUGIN_DIR" && npm install)
    fi

    # Build the webpack bundle
    (cd "$SECURITY_PLUGIN_DIR" && npm run build)

    echo -e "${GREEN}Security Plugin frontend built successfully!${NC}"
fi

# Build Go server
echo -e "${YELLOW}Building Go server...${NC}"
cd "$PROJECT_ROOT"
CGO_ENABLED=1 go build -o /tmp/census-server ./cmd/server

echo -e "${GREEN}Server built successfully!${NC}"
ls -lh /tmp/census-server

if [[ "$BUILD_FRONTEND" == "true" ]]; then
    echo ""
    echo -e "${YELLOW}To use the Next.js frontend, set:${NC}"
    echo "  WEB_DIR=$PROJECT_ROOT/web-next/out"
fi
