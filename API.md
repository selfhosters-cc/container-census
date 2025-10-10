# Container Census API Documentation

## Base URL
`http://localhost:8080/api`

---

## Host Endpoints

### GET /hosts
List all configured hosts.

**Response:**
```json
[
  {
    "id": 1,
    "name": "local",
    "address": "unix:///var/run/docker.sock",
    "description": "Local Docker daemon",
    "enabled": true,
    "created_at": "2025-10-07T14:52:58Z",
    "updated_at": "2025-10-07T14:52:58Z"
  }
]
```

### GET /hosts/{id}
Get a specific host by ID.

### POST /hosts/agent
Add a new agent-based host.

**Request:**
```json
{
  "name": "production-server",
  "address": "http://192.168.1.100:9876",
  "description": "Production Docker host",
  "agent_token": "abc123def456..."
}
```

**Response:**
```json
{
  "id": 2,
  "name": "production-server",
  "address": "http://192.168.1.100:9876",
  "description": "Production Docker host",
  "host_type": "agent",
  "agent_status": "online",
  "enabled": true
}
```

### POST /hosts/agent/test
Test connection to an agent.

**Request:**
```json
{
  "address": "http://192.168.1.100:9876",
  "agent_token": "abc123def456..."
}
```

**Response:**
```json
{
  "success": true,
  "message": "Agent is reachable"
}
```

---

## Container Endpoints

### GET /containers
Get latest containers from all hosts.

**Response:**
```json
[
  {
    "id": "abc123...",
    "name": "my-container",
    "image": "nginx:latest",
    "image_id": "sha256:...",
    "state": "running",
    "status": "Up 2 hours",
    "ports": [...],
    "labels": {...},
    "created": "2025-10-07T12:00:00Z",
    "host_id": 1,
    "host_name": "local",
    "scanned_at": "2025-10-07T14:52:58Z"
  }
]
```

### GET /containers/host/{id}
Get latest containers for a specific host.

### GET /containers/history?start={RFC3339}&end={RFC3339}
Get historical container data within a time range.

**Query Parameters:**
- `start` - Start time (RFC3339 format, defaults to 24 hours ago)
- `end` - End time (RFC3339 format, defaults to now)

---

## Container Management Endpoints

### POST /containers/{host_id}/{container_id}/start
Start a stopped container.

**Response:**
```json
{
  "message": "Container started"
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/containers/1/abc123/start
```

### POST /containers/{host_id}/{container_id}/stop
Stop a running container.

**Query Parameters:**
- `timeout` - Timeout in seconds (default: 10)

**Response:**
```json
{
  "message": "Container stopped"
}
```

**Example:**
```bash
curl -X POST "http://localhost:8080/api/containers/1/abc123/stop?timeout=30"
```

### POST /containers/{host_id}/{container_id}/restart
Restart a container.

**Query Parameters:**
- `timeout` - Timeout in seconds (default: 10)

**Response:**
```json
{
  "message": "Container restarted"
}
```

**Example:**
```bash
curl -X POST "http://localhost:8080/api/containers/1/abc123/restart?timeout=30"
```

### DELETE /containers/{host_id}/{container_id}
Remove a container.

**Query Parameters:**
- `force` - Force removal (default: false)

**Response:**
```json
{
  "message": "Container removed"
}
```

**Example:**
```bash
# Remove stopped container
curl -X DELETE http://localhost:8080/api/containers/1/abc123

# Force remove running container
curl -X DELETE "http://localhost:8080/api/containers/1/abc123?force=true"
```

### GET /containers/{host_id}/{container_id}/logs
Get container logs.

**Query Parameters:**
- `tail` - Number of lines to retrieve (default: 100)

**Response:**
```json
{
  "logs": "2025-10-07 14:52:58 Starting application...\n..."
}
```

**Example:**
```bash
# Get last 50 lines
curl "http://localhost:8080/api/containers/1/abc123/logs?tail=50"
```

---

## Image Endpoints

### GET /images
List all images from all hosts.

**Response:**
```json
{
  "local": {
    "host_id": 1,
    "images": [
      {
        "Id": "sha256:...",
        "RepoTags": ["nginx:latest"],
        "Created": 1696723200,
        "Size": 142000000,
        "VirtualSize": 142000000,
        "Containers": 2
      }
    ]
  }
}
```

### GET /images/host/{id}
List all images for a specific host.

**Response:**
```json
[
  {
    "Id": "sha256:...",
    "RepoTags": ["nginx:latest"],
    "Created": 1696723200,
    "Size": 142000000,
    "VirtualSize": 142000000,
    "Containers": 2
  }
]
```

### DELETE /images/{host_id}/{image_id}
Remove an image.

**Query Parameters:**
- `force` - Force removal (default: false)

**Response:**
```json
{
  "message": "Image removed"
}
```

**Example:**
```bash
# Remove unused image
curl -X DELETE http://localhost:8080/api/images/1/sha256:abc123...

# Force remove image (even if containers are using it)
curl -X DELETE "http://localhost:8080/api/images/1/sha256:abc123...?force=true"
```

### POST /images/host/{id}/prune
Remove all unused images from a host.

**Response:**
```json
{
  "message": "Images pruned",
  "space_reclaimed": 524288000
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/images/host/1/prune
```

---

## Scan Endpoints

### POST /scan
Trigger a manual scan of all hosts.

**Response:**
```json
{
  "message": "Scan triggered"
}
```

### GET /scan/results?limit={N}
Get recent scan results.

**Query Parameters:**
- `limit` - Number of results to return (default: 50)

**Response:**
```json
[
  {
    "id": 1,
    "host_id": 1,
    "host_name": "local",
    "started_at": "2025-10-07T14:52:58Z",
    "completed_at": "2025-10-07T14:52:59Z",
    "success": true,
    "error": "",
    "containers_found": 10
  }
]
```

---

## Health Endpoint

### GET /health
Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "time": "2025-10-07T14:52:58Z"
}
```

---

## Error Responses

All endpoints return standard error responses:

```json
{
  "error": "Error message describing what went wrong"
}
```

**Common HTTP Status Codes:**
- `200 OK` - Success
- `202 Accepted` - Request accepted (async operations)
- `400 Bad Request` - Invalid input
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

---

## Examples

### Start a Container
```bash
curl -X POST http://localhost:8080/api/containers/1/abc123/start
```

### Stop Multiple Containers (using a script)
```bash
#!/bin/bash
HOST_ID=1
for CONTAINER_ID in $(curl -s http://localhost:8080/api/containers | jq -r '.[] | select(.state=="running") | .id'); do
  curl -X POST "http://localhost:8080/api/containers/$HOST_ID/$CONTAINER_ID/stop"
done
```

### Clean Up Unused Images
```bash
curl -X POST http://localhost:8080/api/images/host/1/prune
```

### Monitor Container Logs
```bash
# Get logs for a specific container
curl -s "http://localhost:8080/api/containers/1/abc123/logs?tail=100"
```
