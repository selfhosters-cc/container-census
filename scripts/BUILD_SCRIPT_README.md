# Build All Images Script

A comprehensive, interactive script for building all Container Census Docker images with version management and multi-architecture support.

## Features

- 🎯 **Interactive Version Management** - Choose patch/minor/major or custom version
- 🏗️ **Multi-Architecture Builds** - Build for amd64, arm64, or both
- 📦 **Selective Building** - Build server, agent, telemetry, or all
- 🚀 **Registry Push** - Push to Docker Hub, GHCR, or custom registry
- 📝 **Auto-Generate Compose Files** - Creates docker-compose.yml with all built images
- 🎨 **Colorful Output** - Clear, easy-to-read console output
- ✅ **Version Tracking** - Saves version to `.version` file
- 🔧 **Portable Builds** - Uses default GID for runtime flexibility

## Prerequisites

- Docker with BuildX support
- Docker Compose (for testing)
- Sufficient disk space (~2GB)
- (Optional) Docker Hub or GHCR credentials for pushing

## Installation

The script is located at `scripts/build-all-images.sh` and is already executable.

## Usage

### Basic Usage

```bash
./scripts/build-all-images.sh
```

The script will guide you through:

1. **Version Selection** - Choose how to increment version
2. **Image Selection** - Choose which images to build
3. **Platform Selection** - Choose target architectures
4. **Registry Push** - Optionally push to registry
5. **Compose Generation** - Generate sample docker-compose file

### Example Session

```
═══════════════════════════════════════════════════════════
  Container Census - Multi-Architecture Build Script
═══════════════════════════════════════════════════════════

ℹ Checking prerequisites...
✓ Docker buildx is available
✓ Builder ready
ℹ Current version: 1.2.3

Select version increment:
  1) Patch (1.2.3 → 1.2.4)  - Bug fixes, small changes
  2) Minor (1.2.3 → 1.3.0)  - New features, backward compatible
  3) Major (1.2.3 → 2.0.0)  - Breaking changes
  4) Keep current version (1.2.3)
  5) Enter custom version

Choice [1-5]: 2
✓ Selected version: 1.3.0

Select images to build:
  1) Server (container-census)
  2) Agent (census-agent)
  3) Telemetry Collector (telemetry-collector)
  4) All images

Choice [1-4]: 4

Select target platforms:
  1) linux/amd64 (x86_64 only)
  2) linux/arm64 (ARM64 only)
  3) linux/amd64,linux/arm64 (Both - recommended)

Choice [1-3]: 3

Push to registry? (y/N): y

Select registry:
  1) Docker Hub (username/image)
  2) GitHub Container Registry (ghcr.io/username/image)
  3) Custom registry

Choice [1-3]: 2
GitHub username/org: mycompany

═══════════════════════════════════════════════════════════
  Build Summary
═══════════════════════════════════════════════════════════
Version:   1.3.0
Platforms: linux/amd64,linux/arm64
Images:
  - ✓ container-census
  - ✓ census-agent
  - ✓ telemetry-collector
Registry:  ghcr.io/mycompany

Proceed with build? (y/N): y
```

## Version Management

### Version File

The script maintains a `.version` file in the project root that stores the current version number.

```bash
# View current version
cat .version

# Example content
1.3.0
```

### Version Increment Options

| Option | Description | Example |
|--------|-------------|---------|
| Patch | Bug fixes, small changes | 1.2.3 → 1.2.4 |
| Minor | New features, backward compatible | 1.2.3 → 1.3.0 |
| Major | Breaking changes | 1.2.3 → 2.0.0 |
| Keep | Use current version | 1.2.3 → 1.2.3 |
| Custom | Enter any version | 1.2.3 → 2.5.7 |

## Build Options

### Image Selection

- **Server** - Main Container Census server
- **Agent** - Lightweight agent for remote hosts
- **Telemetry** - Analytics collector
- **All** - Build all three images

### Platform Selection

| Platform | Description | Use Case |
|----------|-------------|----------|
| linux/amd64 | x86_64 only | Most servers, Intel/AMD CPUs |
| linux/arm64 | ARM64 only | Raspberry Pi, ARM servers |
| Both | Multi-arch | Maximum compatibility (recommended) |

### Registry Options

**Docker Hub:**
```
Registry format: username/image
Example: mycompany/container-census:1.3.0
```

**GitHub Container Registry:**
```
Registry format: ghcr.io/username/image
Example: ghcr.io/mycompany/container-census:1.3.0
```

**Custom Registry:**
```
Registry format: registry.example.com/path
Example: registry.example.com/mycompany/container-census:1.3.0
```

## Generated Files

### Docker Compose File

The script can generate a complete docker-compose file for all built images:

```
docker-compose.images-1.3.0.yml
```

Features:
- Uses built image tags
- Sets DOCKER_GID via group_add
- Includes all selected services
- Comprehensive documentation
- Ready to use

### Environment File

Also generates a matching `.env` example:

```
.env.images-1.3.0.example
```

Contents:
- DOCKER_GID configuration
- Service-specific variables
- Comments and examples

## Output Examples

### Successful Build

```
═══════════════════════════════════════════════════════════
  Building container-census:1.3.0
═══════════════════════════════════════════════════════════

ℹ Platforms: linux/amd64,linux/arm64
ℹ Dockerfile: Dockerfile
ℹ Building multi-architecture image...
✓ container-census:1.3.0 built successfully
ℹ Image size: 45.2MB

═══════════════════════════════════════════════════════════
  Build Complete! 🎉
═══════════════════════════════════════════════════════════

Built images:
container-census    1.3.0    abc123def456   2 minutes ago   45.2MB
container-census    latest   abc123def456   2 minutes ago   45.2MB
census-agent        1.3.0    789ghi012jkl   1 minute ago    35.8MB
census-agent        latest   789ghi012jkl   1 minute ago    35.8MB
telemetry-collector 1.3.0    345mno678pqr   30 seconds ago  38.1MB
telemetry-collector latest   345mno678pqr   30 seconds ago  38.1MB

✓ All images built successfully!
ℹ Version: 1.3.0

✓ Images pushed to registry: ghcr.io/mycompany

Generate sample docker-compose.yml for these images? (y/N): y
✓ Generated: docker-compose.images-1.3.0.yml
✓ Generated: .env.images-1.3.0.example

ℹ To use:
  1. cp .env.images-1.3.0.example .env
  2. Edit .env with your settings
  3. docker-compose -f docker-compose.images-1.3.0.yml up -d

✓ Done! 🚀
```

## Advanced Usage

### Build Specific Version

While the script is interactive, you can prepare by:

1. Manually set version in `.version` file
2. Run script and choose "Keep current version"

### Skip Registry Push

Always available - just answer "N" when prompted.

### Build for Single Architecture

Choose option 1 or 2 for platform selection to build only amd64 or arm64.

### Rebuild Same Version

Choose "Keep current version" to rebuild without incrementing.

## Troubleshooting

### BuildX Not Available

```
✗ Docker buildx is not available!
ℹ Install with: docker buildx install
```

**Solution:**
```bash
# Update Docker to latest version
# Or manually install buildx
docker buildx install
```

### Build Failed

```
✗ Build failed for container-census
✗ Build failed! Version not saved.
```

**Common Causes:**
- Network issues during image pull
- Insufficient disk space
- Docker daemon not running
- Syntax errors in Dockerfile

**Solution:**
```bash
# Check Docker status
docker info

# Clean up space
docker system prune -a

# Check logs for specific error
```

### Permission Denied

```
Error: permission denied while trying to connect to Docker daemon
```

**Solution:**
```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Re-login or
newgrp docker
```

### Registry Push Failed

```
Error: authentication required
```

**Solution:**
```bash
# Login to Docker Hub
docker login

# Login to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Login to custom registry
docker login registry.example.com
```

## Best Practices

### Version Numbering

Follow semantic versioning:
- **Major (X.0.0)** - Breaking changes, API changes
- **Minor (0.X.0)** - New features, backward compatible
- **Patch (0.0.X)** - Bug fixes, small changes

### Multi-Architecture Builds

Always build for both architectures unless you have a specific reason not to:
- ✅ Maximum compatibility
- ✅ Works on ARM servers (Raspberry Pi, AWS Graviton)
- ✅ Works on x86 servers (traditional servers, laptops)
- ❌ Slightly longer build time (worth it)

### Registry Organization

Use consistent naming:
```
yourorg/container-census:1.3.0
yourorg/census-agent:1.3.0
yourorg/telemetry-collector:1.3.0
```

### Tagging Strategy

The script creates two tags automatically:
- `1.3.0` - Specific version
- `latest` - Latest build

This allows users to:
```bash
# Pin to specific version (recommended for production)
image: yourorg/container-census:1.3.0

# Use latest (good for development)
image: yourorg/container-census:latest
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Build Images

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Login to GHCR
        uses: docker/login-action@v2
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract version
        id: version
        run: echo "version=${GITHUB_REF#refs/tags/v}" >> $GITHUB_OUTPUT

      - name: Build and push
        run: |
          echo "${{ steps.version.outputs.version }}" > .version
          # Script can be adapted for non-interactive use
          # Or call docker buildx directly with extracted version
```

## Files Created

| File | Purpose |
|------|---------|
| `.version` | Stores current version number |
| `docker-compose.images-X.Y.Z.yml` | Compose file for built images |
| `.env.images-X.Y.Z.example` | Environment variable template |

## Related Documentation

- [BUILD_IMAGES.md](../BUILD_IMAGES.md) - Manual build instructions
- [README.md](../README.md) - Main documentation
- [docker-compose.all-images.yml](../docker-compose.all-images.yml) - Sample compose file

## Support

For issues or questions:
1. Check troubleshooting section above
2. Review [BUILD_IMAGES.md](../BUILD_IMAGES.md)
3. Open a GitHub issue

---

**Happy Building! 🚀**

## Multi-Architecture Build Behavior

### Important: Local Availability

**Single-Platform Builds (amd64 OR arm64):**
- ✅ Images are loaded into local Docker automatically
- ✅ Appear in `docker images` output
- ✅ Can be used immediately with `docker run` or `docker-compose`

**Multi-Platform Builds (amd64 AND arm64):**
- ⚠️  Images are built but NOT loaded locally
- ⚠️  Will NOT appear in `docker images` output
- ⚠️  Stored in build cache only
- ✅ Available after pushing to registry

### Why This Limitation?

Docker's local image store can only hold one platform architecture at a time. Multi-architecture images are "manifest lists" that point to multiple platform-specific images, which can only exist in a registry.

### Solutions for Local Testing

**Option 1: Build for Your Platform Only**
```bash
./scripts/build-all-images.sh
# Choose: Platform → 1 (linux/amd64) or 2 (linux/arm64)
```

**Option 2: Push to Registry and Pull Back**
```bash
# Build and push
./scripts/build-all-images.sh
# Choose: Push to registry → y

# Pull back
docker pull yourorg/container-census:1.3.0
```

**Option 3: Use Local Registry**
```bash
# Start local registry
docker run -d -p 5000:5000 --name registry registry:2

# Build and push to local registry
./scripts/build-all-images.sh
# Registry: 3 (Custom)
# URL: localhost:5000

# Pull back
docker pull localhost:5000/container-census:1.3.0
docker tag localhost:5000/container-census:1.3.0 container-census:1.3.0
```

### Recommended Workflow

For **development/testing:**
- Build single platform for your system
- Fast, immediately available

For **production/distribution:**
- Build multi-platform
- Push to registry (Docker Hub, GHCR, etc.)
- Users pull the correct architecture automatically

