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

# Load configuration from .env file if it exists
SCRIPT_DIR_ENV="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR_ENV/.env" ]; then
    source "$SCRIPT_DIR_ENV/.env"
fi

# Ntfy configuration (can be overridden by .env or environment)
NTFY_SERVER="${NTFY_SERVER:-}"
NTFY_TOPIC="${NTFY_TOPIC:-}"

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

# Function to send ntfy notification (no-op if not configured)
notify() {
    # Skip if ntfy is not configured
    [ -z "$NTFY_SERVER" ] || [ -z "$NTFY_TOPIC" ] && return 0

    local title="$1"
    local message="$2"
    local priority="${3:-default}"
    local tags="${4:-package}"

    curl -s \
        -H "Title: $title" \
        -H "Priority: $priority" \
        -H "Tags: $tags" \
        -d "$message" \
        "https://${NTFY_SERVER}/${NTFY_TOPIC}" > /dev/null 2>&1 || true
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

# Function to build an image for a single platform
build_image_platform() {
    local name=$1
    local dockerfile=$2
    local version=$3
    local platform=$4
    local build_time=$5

    local arch_name=$(echo "$platform" | sed 's/linux\///')
    print_info "Building $name:$version for $arch_name..."
    notify "Building $name ($arch_name)" "Version: $version" "default" "hammer"

    # Build arguments
    local build_args=""
    if [[ "$dockerfile" == "Dockerfile" || "$dockerfile" == "Dockerfile.agent" ]]; then
        build_args="--build-arg DOCKER_GID=999"
    fi
    build_args="$build_args --build-arg BUILD_TIME=$build_time"

    # Build with --load for single platform
    if docker buildx build \
        --platform "$platform" \
        $build_args \
        -t "$name:$version" \
        -t "$name:latest" \
        -f "$dockerfile" \
        --load \
        --progress=plain \
        . ; then
        print_success "$name:$version ($arch_name) built successfully"
        notify "$name ($arch_name) ✓" "Built successfully" "default" "white_check_mark"
        return 0
    else
        print_error "Build failed for $name ($arch_name)"
        notify "$name ($arch_name) ✗" "Build FAILED" "high" "x"
        return 1
    fi
}

# Function to build and push multi-arch image
build_and_push_multiarch() {
    local name=$1
    local dockerfile=$2
    local version=$3
    local platforms=$4
    local registry=$5
    local build_time=$6

    print_info "Building and pushing $registry/$name:$version (multi-arch)..."
    notify "Pushing $name" "Building multi-arch and pushing to $registry" "default" "rocket"

    local build_args=""
    if [[ "$dockerfile" == "Dockerfile" || "$dockerfile" == "Dockerfile.agent" ]]; then
        build_args="--build-arg DOCKER_GID=999"
    fi
    build_args="$build_args --build-arg BUILD_TIME=$build_time"

    if docker buildx build \
        --platform "$platforms" \
        $build_args \
        -t "$registry/$name:$version" \
        -t "$registry/$name:latest" \
        -f "$dockerfile" \
        --push \
        --progress=plain \
        . ; then
        print_success "Pushed to $registry/$name:$version"
        notify "$name pushed ✓" "Pushed to $registry" "default" "white_check_mark"
        return 0
    else
        print_error "Push failed for $name"
        notify "$name push ✗" "Push FAILED" "high" "x"
        return 1
    fi
}

# ==============================================================================
# MAIN SCRIPT - COLLECT ALL OPTIONS UPFRONT
# ==============================================================================

clear
print_header "Container Census - Multi-Architecture Build Script"

# Check prerequisites
print_info "Checking prerequisites..."
check_buildx
setup_builder

# Get current version
CURRENT_VERSION=$(get_current_version)
print_info "Current version: ${CYAN}$CURRENT_VERSION${NC}"

print_header "Configuration - Answer All Questions First"

# ==============================================================================
# Question 1: Version
# ==============================================================================
echo ""
echo -e "${YELLOW}[1/7] Version Selection${NC}"
echo "Select version increment:"
echo -e "  ${GREEN}1${NC}) Patch (${CURRENT_VERSION} → $(increment_version "$CURRENT_VERSION" patch))  - Bug fixes, small changes"
echo -e "  ${GREEN}2${NC}) Minor (${CURRENT_VERSION} → $(increment_version "$CURRENT_VERSION" minor))  - New features, backward compatible"
echo -e "  ${GREEN}3${NC}) Major (${CURRENT_VERSION} → $(increment_version "$CURRENT_VERSION" major))  - Breaking changes"
echo -e "  ${GREEN}4${NC}) Keep current version ($CURRENT_VERSION)"
echo -e "  ${GREEN}5${NC}) Enter custom version"
echo ""
read -p "Choice [1-5]: " version_choice

case $version_choice in
    1) NEW_VERSION=$(increment_version "$CURRENT_VERSION" patch) ;;
    2) NEW_VERSION=$(increment_version "$CURRENT_VERSION" minor) ;;
    3) NEW_VERSION=$(increment_version "$CURRENT_VERSION" major) ;;
    4) NEW_VERSION=$CURRENT_VERSION ;;
    5)
        read -p "Enter version (e.g., 1.2.3): " NEW_VERSION
        if ! [[ $NEW_VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            print_error "Invalid version format. Use X.Y.Z"
            exit 1
        fi
        ;;
    *) print_error "Invalid choice"; exit 1 ;;
esac

print_success "Version: ${GREEN}$NEW_VERSION${NC}"

# ==============================================================================
# Question 2: Images to build
# ==============================================================================
echo ""
echo -e "${YELLOW}[2/7] Image Selection${NC}"
echo "Select images to build:"
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
    1) BUILD_SERVER=true ;;
    2) BUILD_AGENT=true ;;
    3) BUILD_TELEMETRY=true ;;
    4) BUILD_SERVER=true; BUILD_AGENT=true; BUILD_TELEMETRY=true ;;
    *) print_error "Invalid choice"; exit 1 ;;
esac

# ==============================================================================
# Question 3: Agent variant (if building agent)
# ==============================================================================
AGENT_VARIANT=1
if [ "$BUILD_AGENT" = true ]; then
    echo ""
    echo -e "${YELLOW}[3/7] Agent Variant${NC}"
    echo "Which agent variant(s) to build?"
    echo -e "  ${GREEN}1${NC}) Lightweight (no Trivy) - census-agent:latest"
    echo -e "  ${GREEN}2${NC}) With Trivy - census-agent:with-trivy"
    echo -e "  ${GREEN}3${NC}) Both variants"
    echo ""
    read -p "Choice [1-3]: " AGENT_VARIANT
else
    echo ""
    echo -e "${YELLOW}[3/7] Agent Variant${NC} - Skipped (not building agent)"
fi

# ==============================================================================
# Question 4: Platforms
# ==============================================================================
echo ""
echo -e "${YELLOW}[4/7] Platform Selection${NC}"
echo "Select target platforms:"
echo -e "  ${GREEN}1${NC}) linux/amd64 (x86_64 only)"
echo -e "  ${GREEN}2${NC}) linux/arm64 (ARM64 only)"
echo -e "  ${GREEN}3${NC}) linux/amd64,linux/arm64 (Both - recommended)"
echo ""
read -p "Choice [1-3]: " platform_choice

case $platform_choice in
    1) PLATFORMS="linux/amd64" ;;
    2) PLATFORMS="linux/arm64" ;;
    3) PLATFORMS="linux/amd64,linux/arm64" ;;
    *) print_error "Invalid choice"; exit 1 ;;
esac

# ==============================================================================
# Question 5: Registry push
# ==============================================================================
echo ""
echo -e "${YELLOW}[5/7] Registry Push${NC}"
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
        *) print_error "Invalid choice"; exit 1 ;;
    esac
fi

# ==============================================================================
# Question 6: Build Next.js frontend (if building server)
# ==============================================================================
BUILD_FRONTEND=false
if [ "$BUILD_SERVER" = true ]; then
    echo ""
    echo -e "${YELLOW}[6/7] Frontend Build${NC}"
    read -p "Build Next.js frontend? (Y/n): " build_frontend
    if [[ ! $build_frontend =~ ^[Nn]$ ]]; then
        BUILD_FRONTEND=true
    fi
else
    echo ""
    echo -e "${YELLOW}[6/7] Frontend Build${NC} - Skipped (not building server)"
fi

# ==============================================================================
# Question 7: GitHub Release
# ==============================================================================
CREATE_RELEASE=false
if [ "$PUSH_TO_REGISTRY" = true ]; then
    echo ""
    echo -e "${YELLOW}[7/7] GitHub Release${NC}"
    read -p "Create GitHub Release after push? (y/N): " create_release_choice
    if [[ $create_release_choice =~ ^[Yy]$ ]]; then
        CREATE_RELEASE=true
    fi
else
    echo ""
    echo -e "${YELLOW}[7/7] GitHub Release${NC} - Skipped (not pushing to registry)"
fi

# ==============================================================================
# Summary and Confirmation
# ==============================================================================
print_header "Build Summary"
echo -e "Version:   ${GREEN}$NEW_VERSION${NC}"
echo -e "Platforms: ${CYAN}$PLATFORMS${NC}"
echo ""
echo "Images to build:"
[ "$BUILD_SERVER" = true ] && echo -e "  ${GREEN}✓${NC} container-census"
[ "$BUILD_AGENT" = true ] && echo -e "  ${GREEN}✓${NC} census-agent (variant: $AGENT_VARIANT)"
[ "$BUILD_TELEMETRY" = true ] && echo -e "  ${GREEN}✓${NC} telemetry-collector"
echo ""
[ "$BUILD_FRONTEND" = true ] && echo -e "Frontend:  ${GREEN}✓${NC} Build Next.js"
[ "$BUILD_FRONTEND" = false ] && [ "$BUILD_SERVER" = true ] && echo -e "Frontend:  ${YELLOW}○${NC} Skip (use vanilla JS)"
if [ "$PUSH_TO_REGISTRY" = true ]; then
    echo -e "Registry:  ${CYAN}$REGISTRY${NC}"
    [ "$CREATE_RELEASE" = true ] && echo -e "Release:   ${GREEN}✓${NC} Create GitHub Release"
fi
echo ""
if [ -n "$NTFY_SERVER" ] && [ -n "$NTFY_TOPIC" ]; then
    echo -e "Notifications: ${CYAN}https://${NTFY_SERVER}/${NTFY_TOPIC}${NC}"
else
    echo -e "Notifications: ${YELLOW}○${NC} Not configured (see scripts/.env.example)"
fi
echo ""
read -p "Proceed with build? (Y/n): " confirm

if [[ $confirm =~ ^[Nn]$ ]]; then
    print_warning "Build cancelled"
    exit 0
fi

# ==============================================================================
# START BUILD PROCESS
# ==============================================================================

# Save version BEFORE building so it gets embedded in the image
save_version "$NEW_VERSION"

# Notify build start
notify "Build Started 🚀" "Version: $NEW_VERSION | Images: $([ "$BUILD_SERVER" = true ] && echo "server ")$([ "$BUILD_AGENT" = true ] && echo "agent ")$([ "$BUILD_TELEMETRY" = true ] && echo "collector")" "default" "rocket"

print_header "Starting Build Process"

# Get build time once for all images
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
print_info "Build timestamp: $BUILD_TIME"

# Get project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# ==============================================================================
# Build Frontends (if building server)
# ==============================================================================
if [ "$BUILD_SERVER" = true ]; then
    # Build Next.js frontend
    if [ "$BUILD_FRONTEND" = true ]; then
        print_header "Building Next.js Frontend"
        notify "Building Frontend" "Next.js build started" "default" "hammer"

        if [ ! -d "$PROJECT_ROOT/web-next/node_modules" ]; then
            print_info "Installing npm dependencies..."
            (cd "$PROJECT_ROOT/web-next" && npm install)
        fi

        (cd "$PROJECT_ROOT/web-next" && npm run build)
        print_success "Next.js frontend built successfully!"
        notify "Frontend ✓" "Next.js build complete" "default" "white_check_mark"
    fi

    # Build Graph Plugin Frontend
    GRAPH_PLUGIN_DIR="$PROJECT_ROOT/internal/plugins/builtin/graph/frontend"
    if [ -d "$GRAPH_PLUGIN_DIR/src" ]; then
        print_info "Building Graph Plugin frontend..."
        if [ ! -d "$GRAPH_PLUGIN_DIR/node_modules" ]; then
            (cd "$GRAPH_PLUGIN_DIR" && npm install)
        fi
        (cd "$GRAPH_PLUGIN_DIR" && npm run build)
        print_success "Graph Plugin frontend built!"
    fi

    # Build Security Plugin Frontend
    SECURITY_PLUGIN_DIR="$PROJECT_ROOT/internal/plugins/builtin/security/frontend"
    if [ -d "$SECURITY_PLUGIN_DIR/src" ]; then
        print_info "Building Security Plugin frontend..."
        if [ ! -d "$SECURITY_PLUGIN_DIR/node_modules" ]; then
            (cd "$SECURITY_PLUGIN_DIR" && npm install)
        fi
        (cd "$SECURITY_PLUGIN_DIR" && npm run build)
        print_success "Security Plugin frontend built!"
    fi
fi

BUILD_SUCCESS=true

# ==============================================================================
# Build Server
# ==============================================================================
if [ "$BUILD_SERVER" = true ]; then
    print_header "Building Server"

    # Build for each platform
    IFS=',' read -ra PLATFORM_ARRAY <<< "$PLATFORMS"
    for platform in "${PLATFORM_ARRAY[@]}"; do
        if ! build_image_platform "container-census" "Dockerfile" "$NEW_VERSION" "$platform" "$BUILD_TIME"; then
            BUILD_SUCCESS=false
        fi
    done

    # Push multi-arch if requested
    if [ "$PUSH_TO_REGISTRY" = true ] && [ "$BUILD_SUCCESS" = true ]; then
        build_and_push_multiarch "container-census" "Dockerfile" "$NEW_VERSION" "$PLATFORMS" "$REGISTRY" "$BUILD_TIME"
    fi
fi

# ==============================================================================
# Build Agent
# ==============================================================================
if [ "$BUILD_AGENT" = true ]; then
    print_header "Building Agent"

    IFS=',' read -ra PLATFORM_ARRAY <<< "$PLATFORMS"

    case "$AGENT_VARIANT" in
        1)
            # Lightweight only
            for platform in "${PLATFORM_ARRAY[@]}"; do
                if ! build_image_platform "census-agent" "Dockerfile.agent" "$NEW_VERSION" "$platform" "$BUILD_TIME"; then
                    BUILD_SUCCESS=false
                fi
            done
            if [ "$PUSH_TO_REGISTRY" = true ] && [ "$BUILD_SUCCESS" = true ]; then
                build_and_push_multiarch "census-agent" "Dockerfile.agent" "$NEW_VERSION" "$PLATFORMS" "$REGISTRY" "$BUILD_TIME"
            fi
            ;;
        2)
            # With Trivy only
            for platform in "${PLATFORM_ARRAY[@]}"; do
                arch_name=$(echo "$platform" | sed 's/linux\///')
                print_info "Building census-agent:with-trivy for $arch_name..."
                notify "Building agent:with-trivy ($arch_name)" "Version: $NEW_VERSION" "default" "hammer"

                if docker buildx build \
                    --platform "$platform" \
                    --build-arg DOCKER_GID=999 \
                    --build-arg INSTALL_TRIVY=true \
                    --build-arg BUILD_TIME="$BUILD_TIME" \
                    -t "census-agent:with-trivy-$NEW_VERSION" \
                    -t "census-agent:with-trivy" \
                    -f "Dockerfile.agent" \
                    --load \
                    --progress=plain \
                    . ; then
                    print_success "census-agent:with-trivy ($arch_name) built"
                    notify "agent:with-trivy ($arch_name) ✓" "Built successfully" "default" "white_check_mark"
                else
                    BUILD_SUCCESS=false
                    notify "agent:with-trivy ($arch_name) ✗" "Build FAILED" "high" "x"
                fi
            done
            if [ "$PUSH_TO_REGISTRY" = true ] && [ "$BUILD_SUCCESS" = true ]; then
                notify "Pushing agent:with-trivy" "Multi-arch push to $REGISTRY" "default" "rocket"
                docker buildx build \
                    --platform "$PLATFORMS" \
                    --build-arg DOCKER_GID=999 \
                    --build-arg INSTALL_TRIVY=true \
                    --build-arg BUILD_TIME="$BUILD_TIME" \
                    -t "$REGISTRY/census-agent:with-trivy-$NEW_VERSION" \
                    -t "$REGISTRY/census-agent:with-trivy" \
                    -f "Dockerfile.agent" \
                    --push \
                    --progress=plain \
                    .
                notify "agent:with-trivy pushed ✓" "Pushed to $REGISTRY" "default" "white_check_mark"
            fi
            ;;
        3)
            # Both variants
            # Lightweight
            for platform in "${PLATFORM_ARRAY[@]}"; do
                if ! build_image_platform "census-agent" "Dockerfile.agent" "$NEW_VERSION" "$platform" "$BUILD_TIME"; then
                    BUILD_SUCCESS=false
                fi
            done
            if [ "$PUSH_TO_REGISTRY" = true ] && [ "$BUILD_SUCCESS" = true ]; then
                build_and_push_multiarch "census-agent" "Dockerfile.agent" "$NEW_VERSION" "$PLATFORMS" "$REGISTRY" "$BUILD_TIME"
            fi

            # With Trivy
            for platform in "${PLATFORM_ARRAY[@]}"; do
                arch_name=$(echo "$platform" | sed 's/linux\///')
                print_info "Building census-agent:with-trivy for $arch_name..."
                notify "Building agent:with-trivy ($arch_name)" "Version: $NEW_VERSION" "default" "hammer"

                if docker buildx build \
                    --platform "$platform" \
                    --build-arg DOCKER_GID=999 \
                    --build-arg INSTALL_TRIVY=true \
                    --build-arg BUILD_TIME="$BUILD_TIME" \
                    -t "census-agent:with-trivy-$NEW_VERSION" \
                    -t "census-agent:with-trivy" \
                    -f "Dockerfile.agent" \
                    --load \
                    --progress=plain \
                    . ; then
                    print_success "census-agent:with-trivy ($arch_name) built"
                    notify "agent:with-trivy ($arch_name) ✓" "Built successfully" "default" "white_check_mark"
                else
                    BUILD_SUCCESS=false
                    notify "agent:with-trivy ($arch_name) ✗" "Build FAILED" "high" "x"
                fi
            done
            if [ "$PUSH_TO_REGISTRY" = true ] && [ "$BUILD_SUCCESS" = true ]; then
                notify "Pushing agent:with-trivy" "Multi-arch push to $REGISTRY" "default" "rocket"
                docker buildx build \
                    --platform "$PLATFORMS" \
                    --build-arg DOCKER_GID=999 \
                    --build-arg INSTALL_TRIVY=true \
                    --build-arg BUILD_TIME="$BUILD_TIME" \
                    -t "$REGISTRY/census-agent:with-trivy-$NEW_VERSION" \
                    -t "$REGISTRY/census-agent:with-trivy" \
                    -f "Dockerfile.agent" \
                    --push \
                    --progress=plain \
                    .
                notify "agent:with-trivy pushed ✓" "Pushed to $REGISTRY" "default" "white_check_mark"
            fi
            ;;
    esac
fi

# ==============================================================================
# Build Telemetry Collector
# ==============================================================================
if [ "$BUILD_TELEMETRY" = true ]; then
    print_header "Building Telemetry Collector"

    IFS=',' read -ra PLATFORM_ARRAY <<< "$PLATFORMS"
    for platform in "${PLATFORM_ARRAY[@]}"; do
        if ! build_image_platform "telemetry-collector" "Dockerfile.telemetry-collector" "$NEW_VERSION" "$platform" "$BUILD_TIME"; then
            BUILD_SUCCESS=false
        fi
    done

    if [ "$PUSH_TO_REGISTRY" = true ] && [ "$BUILD_SUCCESS" = true ]; then
        build_and_push_multiarch "telemetry-collector" "Dockerfile.telemetry-collector" "$NEW_VERSION" "$PLATFORMS" "$REGISTRY" "$BUILD_TIME"
    fi
fi

# ==============================================================================
# Results
# ==============================================================================
if [ "$BUILD_SUCCESS" = true ]; then
    print_header "Build Complete! 🎉"

    # Show built images
    platform_count=$(echo "$PLATFORMS" | tr ',' '\n' | wc -l)
    if [ "$platform_count" -eq 1 ]; then
        echo "Built images:"
        docker images | grep -E "container-census|census-agent|telemetry-collector" | grep -E "$NEW_VERSION|latest" | head -n 6
        echo ""
    fi

    print_success "All images built successfully!"
    print_info "Version: ${GREEN}$NEW_VERSION${NC}"

    # Create GitHub Release if requested
    if [ "$CREATE_RELEASE" = true ]; then
        echo ""
        print_info "Creating GitHub Release..."
        notify "Creating Release" "GitHub release v$NEW_VERSION" "default" "bookmark"

        if command -v gh &> /dev/null && gh auth status &> /dev/null; then
            if gh release create "v${NEW_VERSION}" \
                --repo selfhosters-cc/container-census \
                --title "v${NEW_VERSION}" \
                --generate-notes; then
                print_success "GitHub release v${NEW_VERSION} created!"
                notify "Release Created ✓" "v$NEW_VERSION published on GitHub" "default" "tada"
            else
                print_warning "Failed to create release (may already exist)"
                notify "Release Warning" "Could not create GitHub release" "default" "warning"
            fi
        else
            print_warning "GitHub CLI not available or not logged in"
        fi
    fi

    # Final notification
    notify "Build Complete 🎉" "Version $NEW_VERSION built successfully!" "default" "tada"
else
    print_header "Build Failed! ❌"
    notify "Build Failed ❌" "Version $NEW_VERSION build failed" "high" "x"
    exit 1
fi

echo ""
print_success "Done! 🚀"
