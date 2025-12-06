#!/bin/bash

# Container Census - Test Docker Build Script
# Builds a local test image and deploys to /opt/docker-compose/census-test/

set -e  # Exit on error

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TEST_COMPOSE_DIR="/opt/docker-compose/census-test"
IMAGE_TAG="container-census:test-local"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_header() {
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo ""
}

# Header
print_header "Container Census - Test Docker Build"

# Get current version
VERSION=$(cat "$PROJECT_ROOT/.version" 2>/dev/null || echo "dev")
print_info "Current version: ${VERSION}"

# Detect Docker socket GID
DOCKER_GID=$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo "999")
print_info "Docker socket GID: ${DOCKER_GID}"

# Build Next.js Frontend
print_header "Building Next.js Frontend"
NEXTJS_DIR="$PROJECT_ROOT/web-next"
if [ -d "$NEXTJS_DIR" ]; then
    print_info "Next.js frontend found"

    # Check if node_modules exists, if not install dependencies
    if [ ! -d "$NEXTJS_DIR/node_modules" ]; then
        print_info "Installing Next.js npm dependencies..."
        (cd "$NEXTJS_DIR" && npm install)
    fi

    # Build Next.js static export
    print_info "Building Next.js static export..."
    (cd "$NEXTJS_DIR" && npm run build)

    print_success "Next.js frontend built successfully!"
else
    print_warning "Next.js frontend not found, will use vanilla JS"
fi

# Build Graph Plugin Frontend
print_header "Building Graph Plugin Frontend"
GRAPH_PLUGIN_DIR="$PROJECT_ROOT/internal/plugins/builtin/graph/frontend"
if [ -d "$GRAPH_PLUGIN_DIR/src" ]; then
    print_info "Graph plugin frontend source found"

    # Check if node_modules exists, if not install dependencies
    if [ ! -d "$GRAPH_PLUGIN_DIR/node_modules" ]; then
        print_info "Installing Graph Plugin npm dependencies..."
        (cd "$GRAPH_PLUGIN_DIR" && npm install)
    fi

    # Build the webpack bundle
    print_info "Building Graph Plugin webpack bundle..."
    (cd "$GRAPH_PLUGIN_DIR" && npm run build)

    print_success "Graph Plugin frontend built successfully!"
else
    print_warning "Graph plugin frontend source not found, skipping"
fi

# Build Docker Image
print_header "Building Docker Image"
print_info "Building ${IMAGE_TAG} for linux/amd64 with Next.js..."
print_info "This may take a few minutes..."

cd "$PROJECT_ROOT"

# Use docker buildx for consistent builds with test Dockerfile
docker buildx build \
    --platform linux/amd64 \
    --build-arg DOCKER_GID=${DOCKER_GID} \
    -f Dockerfile.test \
    -t ${IMAGE_TAG} \
    --load \
    .

print_success "Docker image built: ${IMAGE_TAG} (with Next.js frontend)"

# Stop and Remove Existing Test Container
print_header "Restarting Test Environment"

if [ ! -d "$TEST_COMPOSE_DIR" ]; then
    print_error "Test compose directory not found: ${TEST_COMPOSE_DIR}"
    exit 1
fi

cd "$TEST_COMPOSE_DIR"

# Stop existing container if running
print_info "Stopping existing test container (if running)..."
docker-compose down 2>/dev/null || true

# Remove old image to force using the new one
print_info "Removing old test image (if exists)..."
docker rmi ${IMAGE_TAG} 2>/dev/null || true

# Rebuild the image (critical step - ensures fresh image is used)
print_info "Loading fresh image..."
cd "$PROJECT_ROOT"
docker buildx build \
    --platform linux/amd64 \
    --build-arg DOCKER_GID=${DOCKER_GID} \
    -f Dockerfile.test \
    -t ${IMAGE_TAG} \
    --load \
    . > /dev/null 2>&1

cd "$TEST_COMPOSE_DIR"

# Start fresh container
print_info "Starting fresh test container..."
docker-compose up -d

print_success "Test container started!"

# Show recent logs
print_header "Container Logs (last 20 lines)"
docker-compose logs --tail=20

# Success message
print_header "Test Environment Ready"
print_success "Container Census test instance is running!"
echo ""
print_info "Access the web UI at: ${GREEN}http://localhost:8888${NC}"
print_info "View logs: ${CYAN}cd ${TEST_COMPOSE_DIR} && docker-compose logs -f${NC}"
print_info "Stop container: ${CYAN}cd ${TEST_COMPOSE_DIR} && docker-compose down${NC}"
echo ""
print_info "The test environment uses separate data volumes and runs on port 8888"
print_info "Production instance (if running) continues on port 8887"
echo ""
