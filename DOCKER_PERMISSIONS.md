# Docker Socket Permission Solutions

When running Container Census, the container needs access to the Docker socket to scan containers. Here are the available options:

## ✅ Option 1: Automatic Docker Group Matching (Recommended)

**This is now the default behavior!** The Dockerfile automatically adds the census user to a docker group with the correct GID.

### Using Make (Easiest)

```bash
make docker-build
make docker-run
```

The Makefile automatically detects your host's Docker socket GID.

### Using Docker Compose

```bash
# Set the docker GID and run
DOCKER_GID=$(stat -c '%g' /var/run/docker.sock) docker-compose up -d --build
```

Or use the Makefile wrapper:
```bash
make compose-up
```

### Manual Docker Build

```bash
# Get your host's docker GID
DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)

# Build with that GID
docker build --build-arg DOCKER_GID=$DOCKER_GID -t container-census .

# Run normally
docker run -d \
  --name container-census \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/config/config.yaml:/app/config/config.yaml \
  -v $(pwd)/data:/app/data \
  container-census
```

**Pros:**
- Secure - container runs as non-root
- Works automatically on most systems
- No special host configuration needed

**Cons:**
- Requires rebuilding if moving to different host

---

## Option 2: Add Group at Runtime

Instead of baking the GID into the image, add it at runtime:

```bash
DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)

docker run -d \
  --name container-census \
  --group-add $DOCKER_GID \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/config/config.yaml:/app/config/config.yaml \
  -v $(pwd)/data:/app/data \
  container-census
```

**Pros:**
- Same image works on different hosts
- Still runs as non-root

**Cons:**
- Need to specify group on every run

---

## Option 3: Run as Root (Not Recommended)

```bash
docker run -d \
  --name container-census \
  --user root \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/config/config.yaml:/app/config/config.yaml \
  -v $(pwd)/data:/app/data \
  container-census
```

**Pros:**
- Simple, always works

**Cons:**
- Security risk
- Not recommended for production

---

## Option 4: Scan Remote Docker Hosts (Best for Production)

Instead of mounting the local Docker socket, configure Container Census to scan remote Docker hosts via TCP or SSH:

**Edit config/config.yaml:**

```yaml
hosts:
  # Remote Docker host via TCP with TLS
  - name: production-server
    address: tcp://prod.example.com:2376
    description: Production Docker host

  # Remote Docker host via SSH
  - name: staging-server
    address: ssh://user@staging.example.com
    description: Staging environment
```

**Pros:**
- No Docker socket access needed
- Most secure for production
- Can scan multiple remote hosts

**Cons:**
- Requires Docker API to be exposed on remote hosts (with proper security)
- Or requires SSH access

---

## Verification

After starting the container, check the logs:

```bash
docker logs container-census
```

**Success looks like:**
```
2025/10/07 14:52:58 Scan completed for host local: found 10 containers
```

**Permission denied looks like:**
```
2025/10/07 14:30:45 Scan failed for host local: permission denied while trying to connect to the Docker daemon socket
```

## Testing

Access the web UI: **http://localhost:8080**

Test the API:
```bash
curl http://localhost:8080/api/health
curl http://localhost:8080/api/containers
curl http://localhost:8080/api/hosts
```
