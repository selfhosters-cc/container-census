# Container Census

A Go-based tool that scans configured Docker hosts and tracks all running containers. Container information is timestamped and stored in a database, accessible through a web frontend. The entire stack runs in a single container.

## Features

- **Multi-host scanning**: Monitor multiple Docker hosts from a single dashboard
- **Automatic scanning**: Configurable periodic scans (default: every 5 minutes)
- **Historical tracking**: All container states are timestamped and stored
- **Web UI**: Clean, responsive web interface with real-time updates
- **REST API**: Full API access to all container and host data
- **Single container deployment**: Everything runs in one lightweight container
- **Multiple connection types**: Unix socket, TCP, and SSH connections

## Quick Start

### Using Docker

1. **Create a configuration file:**

```bash
mkdir -p config
cp config/config.yaml.example config/config.yaml
# Edit config/config.yaml to add your Docker hosts
```

2. **Run with Docker:**

```bash
docker build -t container-census .

docker run -d \
  --name container-census \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/config/config.yaml:/app/config/config.yaml \
  -v $(pwd)/data:/app/data \
  container-census
```

3. **Access the web UI:**

Open your browser to http://localhost:8080

### Using Docker Compose

Create a `docker-compose.yml`:

```yaml
version: '3.8'

services:
  container-census:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./config/config.yaml:/app/config/config.yaml
      - ./data:/app/data
    restart: unless-stopped
```

Run:

```bash
docker-compose up -d
```

## Configuration

Edit [config/config.yaml](config/config.yaml):

```yaml
database:
  path: ./data/census.db

server:
  host: 0.0.0.0
  port: 8080

scanner:
  interval_seconds: 300  # Scan every 5 minutes
  timeout_seconds: 30    # Timeout for each scan

hosts:
  # Local Docker daemon
  - name: local
    address: unix:///var/run/docker.sock
    description: Local Docker daemon

  # Remote Docker host via TCP
  - name: remote-host
    address: tcp://192.168.1.100:2376
    description: Remote Docker host

  # Remote Docker host via SSH
  - name: ssh-host
    address: ssh://user@192.168.1.101
    description: Remote host over SSH
```

### Connection Types

- **Unix Socket**: `unix:///var/run/docker.sock` (local Docker daemon)
- **TCP**: `tcp://hostname:2376` (requires Docker API exposed)
- **SSH**: `ssh://user@hostname` (requires SSH access and Docker installed)
- **Local**: Use empty string or `"local"` for local daemon with default settings

## API Endpoints

### Hosts

- `GET /api/hosts` - List all configured hosts
- `GET /api/hosts/{id}` - Get specific host details

### Containers

- `GET /api/containers` - Get latest containers from all hosts
- `GET /api/containers/host/{id}` - Get containers for specific host
- `GET /api/containers/history?start=TIME&end=TIME` - Get historical container data

### Scanning

- `POST /api/scan` - Trigger a manual scan
- `GET /api/scan/results?limit=N` - Get recent scan results

### Health

- `GET /api/health` - Health check endpoint

## Building from Source

### Prerequisites

- Go 1.21 or later
- SQLite3

### Build

```bash
# Clone the repository
git clone https://github.com/yourusername/container-census.git
cd container-census

# Download dependencies
go mod download

# Build
go build -o census ./cmd/server

# Run
./census
```

## Architecture

```
container-census/
├── cmd/server/          # Main application entry point
├── internal/
│   ├── api/            # REST API handlers
│   ├── config/         # Configuration management
│   ├── models/         # Data models
│   ├── scanner/        # Docker host scanning logic
│   └── storage/        # Database operations
├── web/                # Frontend (HTML/CSS/JS)
├── config/             # Configuration files
├── Dockerfile          # Multi-stage Docker build
└── README.md
```

### Tech Stack

- **Backend**: Go 1.21
- **Database**: SQLite3
- **API Router**: Gorilla Mux
- **Docker Client**: Official Docker SDK for Go
- **Frontend**: Vanilla JavaScript (no framework)
- **Container**: Alpine Linux (final image ~30MB)

## Security Considerations

### Docker Socket Access

When mounting `/var/run/docker.sock`, the container has full access to the Docker daemon. Consider:

- Running on a dedicated monitoring host
- Using Docker TCP with TLS certificates for remote hosts
- Using SSH connections for better security
- Implementing network segmentation

### Remote Connections

For TCP connections:
- Use TLS certificates (`tcp://host:2376` with cert authentication)
- Restrict access with firewall rules
- Use SSH tunneling for encrypted connections

For SSH connections:
- Use SSH key authentication
- Ensure proper SSH key management
- Consider using dedicated service accounts

## Development

### Project Structure

- `cmd/server/main.go` - Application entry point with initialization and periodic scanning
- `internal/scanner/` - Docker host connection and container discovery
- `internal/storage/` - SQLite database operations with full CRUD
- `internal/api/` - HTTP handlers and routing
- `internal/models/` - Data structures shared across packages
- `web/` - Static frontend files served by the Go application

### Adding New Features

1. Update models in `internal/models/models.go`
2. Add database operations in `internal/storage/db.go`
3. Implement API handlers in `internal/api/handlers.go`
4. Update frontend in `web/` directory

## Troubleshooting

### Cannot connect to Docker daemon

- Ensure Docker socket is mounted: `-v /var/run/docker.sock:/var/run/docker.sock`
- Check socket permissions
- Verify Docker is running on the host

### Permission denied

- The container runs as non-root user (UID 1000)
- Ensure mounted volumes have correct permissions
- For Docker socket, user needs to be in `docker` group on host

### Remote host connection fails

- Verify network connectivity
- Check firewall rules
- For TCP: Ensure Docker API is exposed on remote host
- For SSH: Verify SSH key authentication works

### Database errors

- Ensure data directory is writable
- Check disk space
- Verify SQLite3 is properly compiled (CGO_ENABLED=1)

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
