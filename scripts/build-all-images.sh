#!/bin/bash

# Container Census - Multi-Architecture Image Build Script
# Builds server, agent, and/or telemetry-collector images with version management

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Version file location
VERSION_FILE="./.version"

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

# Function to get current version
get_current_version() {
    if [ -f "$VERSION_FILE" ]; then
        cat "$VERSION_FILE"
    else
        echo "0.0.0"
    fi
}

# Function to parse version
parse_version() {
    local version=$1
    IFS='.' read -r MAJOR MINOR PATCH <<< "$version"
}

# Function to increment version
increment_version() {
    local current=$1
    local increment=$2

    parse_version "$current"

    case $increment in
        major)
            MAJOR=$((MAJOR + 1))
            MINOR=0
            PATCH=0
            ;;
        minor)
            MINOR=$((MINOR + 1))
            PATCH=0
            ;;
        patch)
            PATCH=$((PATCH + 1))
            ;;
    esac

    echo "${MAJOR}.${MINOR}.${PATCH}"
}

# Function to save version
save_version() {
    echo "$1" > "$VERSION_FILE"
    print_success "Version saved to $VERSION_FILE"
}

# Function to check if buildx is available
check_buildx() {
    if ! docker buildx version &> /dev/null; then
        print_error "Docker buildx is not available!"
        print_info "Install with: docker buildx install"
        exit 1
    fi
    print_success "Docker buildx is available"
}

# Function to create/use buildx builder
setup_builder() {
    local builder_name="container-census-builder"

    if ! docker buildx inspect "$builder_name" &> /dev/null; then
        print_info "Creating buildx builder: $builder_name"
        docker buildx create --name "$builder_name" --use --bootstrap
    else
        print_info "Using existing buildx builder: $builder_name"
        docker buildx use "$builder_name"
    fi

    print_success "Builder ready"
}

# Function to build an image
build_image() {
    local name=$1
    local dockerfile=$2
    local version=$3
    local platforms=${4:-"linux/amd64,linux/arm64"}

    print_header "Building $name:$version"

    print_info "Platforms: $platforms"
    print_info "Dockerfile: $dockerfile"

    # Build arguments
    local build_args=""
    if [[ "$dockerfile" == "Dockerfile" || "$dockerfile" == "Dockerfile.agent" ]]; then
        # Use default GID for portability (runtime override via group_add)
        build_args="--build-arg DOCKER_GID=999"
    fi

    # Determine if this is a multi-platform build
    local platform_count=$(echo "$platforms" | tr ',' '\n' | wc -l)
    local load_flag=""

    if [ "$platform_count" -eq 1 ]; then
        # Single platform - can use --load to load into local Docker
        load_flag="--load"
        print_info "Building single-platform image (will be available locally)..."
    else
        # Multi-platform - cannot use --load
        # Image will be in build cache but not in 'docker images' until pushed
        print_info "Building multi-platform image (cache only - use --push to make available)..."
        print_warning "Multi-arch images won't appear in 'docker images' until pushed to registry"
    fi

    # Build
    docker buildx build \
        --platform "$platforms" \
        $build_args \
        -t "$name:$version" \
        -t "$name:latest" \
        -f "$dockerfile" \
        $load_flag \
        --progress=plain \
        . || {
            print_error "Build failed for $name"
            return 1
        }

    print_success "$name:$version built successfully"

    # Show image size (only works for single platform with --load)
    if [ "$platform_count" -eq 1 ]; then
        local size=$(docker images "$name:$version" --format "{{.Size}}" 2>/dev/null | head -n1)
        if [ -n "$size" ]; then
            print_info "Image size: $size"
        fi
    else
        print_info "Image built in cache (not loaded locally)"
        print_info "To use locally, either:"
        print_info "  1. Build for single platform, or"
        print_info "  2. Push to registry and pull back"
    fi

    return 0
}

# Function to build and optionally push
build_and_push() {
    local name=$1
    local dockerfile=$2
    local version=$3
    local platforms=$4
    local registry=$5

    if [ -n "$registry" ]; then
        print_header "Building and Pushing $registry/$name:$version"

        local build_args=""
        if [[ "$dockerfile" == "Dockerfile" || "$dockerfile" == "Dockerfile.agent" ]]; then
            build_args="--build-arg DOCKER_GID=999"
        fi

        docker buildx build \
            --platform "$platforms" \
            $build_args \
            -t "$registry/$name:$version" \
            -t "$registry/$name:latest" \
            -f "$dockerfile" \
            --push \
            --progress=plain \
            .

        print_success "Pushed to $registry/$name:$version"
    fi
}

# Main script starts here
clear
print_header "Container Census - Multi-Architecture Build Script"

# Check prerequisites
print_info "Checking prerequisites..."
check_buildx
setup_builder

# Get current version
CURRENT_VERSION=$(get_current_version)
print_info "Current version: ${CYAN}$CURRENT_VERSION${NC}"

# Ask for version increment
echo ""
echo "Select version increment:"
echo -e "  ${GREEN}1${NC}) Patch (${CURRENT_VERSION} → $(increment_version "$CURRENT_VERSION" patch))  - Bug fixes, small changes"
echo -e "  ${GREEN}2${NC}) Minor (${CURRENT_VERSION} → $(increment_version "$CURRENT_VERSION" minor))  - New features, backward compatible"
echo -e "  ${GREEN}3${NC}) Major (${CURRENT_VERSION} → $(increment_version "$CURRENT_VERSION" major))  - Breaking changes"
echo -e "  ${GREEN}4${NC}) Keep current version ($CURRENT_VERSION)"
echo -e "  ${GREEN}5${NC}) Enter custom version"
echo ""
read -p "Choice [1-5]: " version_choice

case $version_choice in
    1)
        NEW_VERSION=$(increment_version "$CURRENT_VERSION" patch)
        ;;
    2)
        NEW_VERSION=$(increment_version "$CURRENT_VERSION" minor)
        ;;
    3)
        NEW_VERSION=$(increment_version "$CURRENT_VERSION" major)
        ;;
    4)
        NEW_VERSION=$CURRENT_VERSION
        ;;
    5)
        read -p "Enter version (e.g., 1.2.3): " NEW_VERSION
        if ! [[ $NEW_VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            print_error "Invalid version format. Use X.Y.Z"
            exit 1
        fi
        ;;
    *)
        print_error "Invalid choice"
        exit 1
        ;;
esac

print_success "Selected version: ${GREEN}$NEW_VERSION${NC}"

# Select images to build
echo ""
print_info "Select images to build:"
echo -e "  ${GREEN}1${NC}) Server (container-census)"
echo -e "  ${GREEN}2${NC}) Agent (census-agent)"
echo -e "  ${GREEN}3${NC}) Telemetry Collector (telemetry-collector)"
echo -e "  ${GREEN}4${NC}) All images"
echo ""
read -p "Choice [1-4]: " image_choice

BUILD_SERVER=false
BUILD_AGENT=false
BUILD_TELEMETRY=false

case $image_choice in
    1)
        BUILD_SERVER=true
        ;;
    2)
        BUILD_AGENT=true
        ;;
    3)
        BUILD_TELEMETRY=true
        ;;
    4)
        BUILD_SERVER=true
        BUILD_AGENT=true
        BUILD_TELEMETRY=true
        ;;
    *)
        print_error "Invalid choice"
        exit 1
        ;;
esac

# Select platforms
echo ""
print_info "Select target platforms:"
echo -e "  ${GREEN}1${NC}) linux/amd64 (x86_64 only)"
echo -e "  ${GREEN}2${NC}) linux/arm64 (ARM64 only)"
echo -e "  ${GREEN}3${NC}) linux/amd64,linux/arm64 (Both - recommended)"
echo ""
read -p "Choice [1-3]: " platform_choice

case $platform_choice in
    1)
        PLATFORMS="linux/amd64"
        ;;
    2)
        PLATFORMS="linux/arm64"
        ;;
    3)
        PLATFORMS="linux/amd64,linux/arm64"
        ;;
    *)
        print_error "Invalid choice"
        exit 1
        ;;
esac

# Ask about registry push
echo ""
read -p "Push to registry? (y/N): " push_choice
PUSH_TO_REGISTRY=false
REGISTRY=""

if [[ $push_choice =~ ^[Yy]$ ]]; then
    PUSH_TO_REGISTRY=true
    echo ""
    echo "Select registry:"
    echo -e "  ${GREEN}1${NC}) Docker Hub (username/image)"
    echo -e "  ${GREEN}2${NC}) GitHub Container Registry (ghcr.io/username/image)"
    echo -e "  ${GREEN}3${NC}) Custom registry"
    echo ""
    read -p "Choice [1-3]: " registry_choice

    case $registry_choice in
        1)
            read -p "Docker Hub username: " username
            REGISTRY="$username"
            ;;
        2)
            read -p "GitHub username/org: " username
            REGISTRY="ghcr.io/$username"
            ;;
        3)
            read -p "Custom registry URL (e.g., registry.example.com/path): " custom_registry
            REGISTRY="$custom_registry"
            ;;
        *)
            print_error "Invalid choice"
            exit 1
            ;;
    esac

    print_info "Will push to: $REGISTRY"
fi

# Summary
print_header "Build Summary"
echo -e "Version:   ${GREEN}$NEW_VERSION${NC}"
echo -e "Platforms: ${CYAN}$PLATFORMS${NC}"
echo "Images:"
[ "$BUILD_SERVER" = true ] && echo -e "  - ${GREEN}✓${NC} container-census"
[ "$BUILD_AGENT" = true ] && echo -e "  - ${GREEN}✓${NC} census-agent"
[ "$BUILD_TELEMETRY" = true ] && echo -e "  - ${GREEN}✓${NC} telemetry-collector"
if [ "$PUSH_TO_REGISTRY" = true ]; then
    echo -e "Registry:  ${CYAN}$REGISTRY${NC}"
fi
echo ""
read -p "Proceed with build? (y/N): " confirm

if [[ ! $confirm =~ ^[Yy]$ ]]; then
    print_warning "Build cancelled"
    exit 0
fi

# Start building
print_header "Starting Build Process"

BUILD_SUCCESS=true

# Build server
if [ "$BUILD_SERVER" = true ]; then
    if build_image "container-census" "Dockerfile" "$NEW_VERSION" "$PLATFORMS"; then
        if [ "$PUSH_TO_REGISTRY" = true ]; then
            build_and_push "container-census" "Dockerfile" "$NEW_VERSION" "$PLATFORMS" "$REGISTRY"
        fi
    else
        BUILD_SUCCESS=false
    fi
fi

# Build agent
if [ "$BUILD_AGENT" = true ]; then
    if build_image "census-agent" "Dockerfile.agent" "$NEW_VERSION" "$PLATFORMS"; then
        if [ "$PUSH_TO_REGISTRY" = true ]; then
            build_and_push "census-agent" "Dockerfile.agent" "$NEW_VERSION" "$PLATFORMS" "$REGISTRY"
        fi
    else
        BUILD_SUCCESS=false
    fi
fi

# Build telemetry collector
if [ "$BUILD_TELEMETRY" = true ]; then
    if build_image "telemetry-collector" "Dockerfile.telemetry-collector" "$NEW_VERSION" "$PLATFORMS"; then
        if [ "$PUSH_TO_REGISTRY" = true ]; then
            build_and_push "telemetry-collector" "Dockerfile.telemetry-collector" "$NEW_VERSION" "$PLATFORMS" "$REGISTRY"
        fi
    else
        BUILD_SUCCESS=false
    fi
fi

# Save version if build succeeded
if [ "$BUILD_SUCCESS" = true ]; then
    save_version "$NEW_VERSION"

    print_header "Build Complete! 🎉"

    # Show built images (only visible for single-platform builds)
    platform_count=$(echo "$PLATFORMS" | tr ',' '\n' | wc -l)
    if [ "$platform_count" -eq 1 ]; then
        echo "Built images:"
        docker images | grep -E "container-census|census-agent|telemetry-collector" | grep -E "$NEW_VERSION|latest" | head -n 6
        echo ""
    fi

    print_success "All images built successfully!"
    print_info "Version: ${GREEN}$NEW_VERSION${NC}"

    if [ "$PUSH_TO_REGISTRY" = true ]; then
        echo ""
        print_success "Images pushed to registry: $REGISTRY"
    elif [ "$platform_count" -gt 1 ]; then
        echo ""
        print_warning "Multi-architecture images are in build cache only"
        print_info "To use these images locally:"
        echo "  1. Push to a registry and pull back, OR"
        echo "  2. Re-build for single platform (option 1 or 2)"
    fi

    # Generate docker-compose file
    echo ""
    read -p "Generate sample docker-compose.yml for these images? (y/N): " gen_compose
    if [[ $gen_compose =~ ^[Yy]$ ]]; then
        COMPOSE_FILE="docker-compose.images-${NEW_VERSION}.yml"

        cat > "$COMPOSE_FILE" << EOF
# Container Census - Pre-built Images Deployment
# Version: ${NEW_VERSION}
# Generated: $(date '+%Y-%m-%d %H:%M:%S')

version: '3.8'

services:
EOF

        if [ "$BUILD_SERVER" = true ]; then
            server_image="container-census:${NEW_VERSION}"
            if [ "$PUSH_TO_REGISTRY" = true ]; then
                server_image="${REGISTRY}/container-census:${NEW_VERSION}"
            fi

            cat >> "$COMPOSE_FILE" << EOF
  # Container Census Server
  census-server:
    image: ${server_image}
    container_name: container-census
    restart: unless-stopped

    # Runtime Docker socket GID configuration
    group_add:
      - "\${DOCKER_GID:-999}"

    ports:
      - "8080:8080"

    volumes:
      # Docker socket for scanning local containers
      - /var/run/docker.sock:/var/run/docker.sock
      # Persistent data directory
      - ./data:/app/data
      # Optional: Mount config file
      # - ./config/config.yaml:/app/config/config.yaml

    environment:
      TZ: \${TZ:-UTC}
      # CONFIG_PATH: /app/config/config.yaml

    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 10s

    networks:
      - census-network

EOF
        fi

        if [ "$BUILD_AGENT" = true ]; then
            agent_image="census-agent:${NEW_VERSION}"
            if [ "$PUSH_TO_REGISTRY" = true ]; then
                agent_image="${REGISTRY}/census-agent:${NEW_VERSION}"
            fi

            cat >> "$COMPOSE_FILE" << EOF
  # Container Census Agent
  census-agent:
    image: ${agent_image}
    container_name: census-agent
    restart: unless-stopped

    # Runtime Docker socket GID configuration
    group_add:
      - "\${DOCKER_GID:-999}"

    ports:
      - "9876:9876"

    volumes:
      # Docker socket for local container management
      - /var/run/docker.sock:/var/run/docker.sock

    environment:
      API_TOKEN: \${AGENT_API_TOKEN:-}
      PORT: 9876
      TZ: \${TZ:-UTC}

    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:9876/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s

    networks:
      - census-network

EOF
        fi

        if [ "$BUILD_TELEMETRY" = true ]; then
            telemetry_image="telemetry-collector:${NEW_VERSION}"
            if [ "$PUSH_TO_REGISTRY" = true ]; then
                telemetry_image="${REGISTRY}/telemetry-collector:${NEW_VERSION}"
            fi

            cat >> "$COMPOSE_FILE" << EOF
  # Telemetry Collector (requires PostgreSQL)
  telemetry-postgres:
    image: postgres:15-alpine
    container_name: telemetry-postgres
    restart: unless-stopped

    environment:
      POSTGRES_DB: telemetry
      POSTGRES_USER: \${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: \${POSTGRES_PASSWORD:-postgres}
      PGDATA: /var/lib/postgresql/data/pgdata

    volumes:
      - telemetry-db:/var/lib/postgresql/data

    networks:
      - census-network

    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  telemetry-collector:
    image: ${telemetry_image}
    container_name: telemetry-collector
    restart: unless-stopped

    ports:
      - "8081:8081"

    environment:
      DATABASE_URL: postgres://\${POSTGRES_USER:-postgres}:\${POSTGRES_PASSWORD:-postgres}@telemetry-postgres:5432/telemetry?sslmode=disable
      PORT: 8081
      API_KEY: \${TELEMETRY_API_KEY:-}
      TZ: \${TZ:-UTC}

    depends_on:
      telemetry-postgres:
        condition: service_healthy

    networks:
      - census-network

    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8081/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 10s

EOF
        fi

        cat >> "$COMPOSE_FILE" << EOF
networks:
  census-network:
    name: census-network
    driver: bridge

EOF

        if [ "$BUILD_TELEMETRY" = true ]; then
            cat >> "$COMPOSE_FILE" << EOF
volumes:
  telemetry-db:
    name: telemetry-db

EOF
        fi

        cat >> "$COMPOSE_FILE" << EOF
# ==============================================================================
# Quick Start
# ==============================================================================
#
# 1. Create .env file:
#    echo "DOCKER_GID=\$(stat -c '%g' /var/run/docker.sock)" > .env
#
# 2. Start services:
#    docker-compose -f ${COMPOSE_FILE} up -d
#
# 3. Access services:
EOF

        if [ "$BUILD_SERVER" = true ]; then
            cat >> "$COMPOSE_FILE" << EOF
#    - Server: http://localhost:8080
EOF
        fi

        if [ "$BUILD_AGENT" = true ]; then
            cat >> "$COMPOSE_FILE" << EOF
#    - Agent: http://localhost:9876 (get token from logs)
EOF
        fi

        if [ "$BUILD_TELEMETRY" = true ]; then
            cat >> "$COMPOSE_FILE" << EOF
#    - Analytics: http://localhost:8081
EOF
        fi

        cat >> "$COMPOSE_FILE" << EOF
#
# ==============================================================================
# Environment Variables
# ==============================================================================
#
# Required:
#   DOCKER_GID            - Docker socket GID (auto-detect: stat -c '%g' /var/run/docker.sock)
#
# Optional:
#   TZ                    - Timezone (default: UTC)
EOF

        if [ "$BUILD_AGENT" = true ]; then
            cat >> "$COMPOSE_FILE" << EOF
#   AGENT_API_TOKEN       - Agent API token (auto-generated if not set)
EOF
        fi

        if [ "$BUILD_TELEMETRY" = true ]; then
            cat >> "$COMPOSE_FILE" << EOF
#   POSTGRES_USER         - PostgreSQL username (default: postgres)
#   POSTGRES_PASSWORD     - PostgreSQL password (default: postgres)
#   TELEMETRY_API_KEY     - Telemetry API key (optional)
EOF
        fi

        cat >> "$COMPOSE_FILE" << EOF
#
# ==============================================================================
EOF

        print_success "Generated: ${GREEN}$COMPOSE_FILE${NC}"

        # Generate .env.example
        ENV_EXAMPLE_FILE=".env.images-${NEW_VERSION}.example"
        cat > "$ENV_EXAMPLE_FILE" << EOF
# Container Census - Environment Variables
# Version: ${NEW_VERSION}

# Docker socket GID (required for server and agent)
# Auto-detect with: stat -c '%g' /var/run/docker.sock
DOCKER_GID=999

# Timezone
TZ=UTC
EOF

        if [ "$BUILD_AGENT" = true ]; then
            cat >> "$ENV_EXAMPLE_FILE" << EOF

# Agent API Token (leave empty for auto-generation)
AGENT_API_TOKEN=
EOF
        fi

        if [ "$BUILD_TELEMETRY" = true ]; then
            cat >> "$ENV_EXAMPLE_FILE" << EOF

# PostgreSQL Configuration
POSTGRES_USER=postgres
POSTGRES_PASSWORD=change-this-password

# Telemetry API Key (optional)
TELEMETRY_API_KEY=
EOF
        fi

        print_success "Generated: ${GREEN}$ENV_EXAMPLE_FILE${NC}"

        echo ""
        print_info "To use:"
        echo "  1. cp $ENV_EXAMPLE_FILE .env"
        echo "  2. Edit .env with your settings"
        echo "  3. docker-compose -f $COMPOSE_FILE up -d"
    fi

else
    print_error "Build failed! Version not saved."
    exit 1
fi

echo ""
print_success "Done! 🚀"
