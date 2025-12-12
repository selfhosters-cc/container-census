#!/bin/bash

# Deploy Agent Script
# Builds agent Docker image and deploys to ubuntu3

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
REMOTE_HOST="ubuntu3"
REMOTE_COMPOSE_DIR="/opt/docker-compose"
IMAGE_NAME="census-agent"
DOCKER_GID=$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo "999")

# Read version
VERSION=$(cat .version 2>/dev/null || echo "dev")

echo -e "${YELLOW}Building and deploying Census Agent v${VERSION}...${NC}"
echo ""

# Ask which agent variant to build
echo -e "${YELLOW}Which agent variant to build?${NC}"
echo -e "  ${GREEN}1${NC}) Lightweight (no Trivy) - faster, smaller (~20MB)"
echo -e "  ${GREEN}2${NC}) With Trivy - vulnerability scanning capability (~400MB)"
echo ""
read -p "Choice [1-2] (default: 1): " AGENT_VARIANT
AGENT_VARIANT=${AGENT_VARIANT:-1}

# Set build arguments and image tags based on choice
if [ "$AGENT_VARIANT" = "2" ]; then
    echo -e "${GREEN}Building agent WITH Trivy...${NC}"
    BUILD_ARGS="--build-arg DOCKER_GID=${DOCKER_GID} --build-arg INSTALL_TRIVY=true"
    IMAGE_TAG="with-trivy"
else
    echo -e "${GREEN}Building lightweight agent (no Trivy)...${NC}"
    BUILD_ARGS="--build-arg DOCKER_GID=${DOCKER_GID}"
    IMAGE_TAG="latest"
fi

# Step 1: Build the agent Docker image
echo ""
echo -e "${YELLOW}Step 1/5: Building agent Docker image...${NC}"
docker buildx build \
    --platform linux/amd64 \
    ${BUILD_ARGS} \
    -f Dockerfile.agent \
    -t ${IMAGE_NAME}:${VERSION} \
    -t ${IMAGE_NAME}:${IMAGE_TAG} \
    --load \
    .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Agent image built successfully${NC}"
else
    echo -e "${RED}✗ Failed to build agent image${NC}"
    exit 1
fi

# Step 2: Save the image to a tar file
echo ""
echo -e "${YELLOW}Step 2/5: Saving image to tar file...${NC}"
docker save ${IMAGE_NAME}:${IMAGE_TAG} -o /tmp/${IMAGE_NAME}.tar

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Image saved to /tmp/${IMAGE_NAME}.tar${NC}"
else
    echo -e "${RED}✗ Failed to save image${NC}"
    exit 1
fi

# Step 3: Copy the tar file to ubuntu3
echo ""
echo -e "${YELLOW}Step 3/5: Copying image to ${REMOTE_HOST}...${NC}"
scp /tmp/${IMAGE_NAME}.tar ${REMOTE_HOST}:/tmp/

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Image copied to ${REMOTE_HOST}${NC}"
else
    echo -e "${RED}✗ Failed to copy image to ${REMOTE_HOST}${NC}"
    exit 1
fi

# Step 4: Load the image on ubuntu3 and restart the agent
echo ""
echo -e "${YELLOW}Step 4/5: Loading image and restarting agent on ${REMOTE_HOST}...${NC}"
ssh ${REMOTE_HOST} << EOF
    echo "Loading image..."
    docker load -i /tmp/census-agent.tar

    echo "Tagging image for docker-compose..."
    # Tag the loaded image to match what docker-compose expects
    docker tag ${IMAGE_NAME}:${IMAGE_TAG} ghcr.io/selfhosters-cc/census-agent:latest

    echo "Recreating agent container with new image..."
    cd /opt/docker-compose
    docker compose up -d census-agent

    echo "Cleaning up tar file..."
    rm /tmp/census-agent.tar

    echo "Waiting for agent to be healthy..."
    sleep 3

    # Check if agent is running
    if docker ps | grep -q census-agent; then
        echo "✓ Agent container is running"
    else
        echo "✗ Agent container is not running"
        exit 1
    fi
EOF

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Agent restarted successfully on ${REMOTE_HOST}${NC}"
else
    echo -e "${RED}✗ Failed to restart agent on ${REMOTE_HOST}${NC}"
    exit 1
fi

# Step 5: Cleanup local tar file
echo ""
echo -e "${YELLOW}Step 5/5: Cleaning up local tar file...${NC}"
rm /tmp/${IMAGE_NAME}.tar
echo -e "${GREEN}✓ Cleanup complete${NC}"

# Final status check
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Deployment Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "Agent version: ${YELLOW}${VERSION}${NC}"
echo -e "Agent variant: ${YELLOW}${IMAGE_TAG}${NC}"
echo -e "Deployed to: ${YELLOW}${REMOTE_HOST}${NC}"
echo ""
echo -e "You can check the agent status with:"
echo -e "  ${YELLOW}ssh ${REMOTE_HOST} 'docker ps | grep census-agent'${NC}"
echo -e "  ${YELLOW}ssh ${REMOTE_HOST} 'docker logs census-agent'${NC}"
echo ""
