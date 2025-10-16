# Container Census

A Go-based tool that scans configured Docker hosts and tracks all running containers. Container information is timestamped and stored in a database, accessible through a web frontend. The entire stack runs in a single container.

## Features

- **Multi-host scanning**: Monitor multiple Docker hosts from a single dashboard
- **Lightweight Agent**: Deploy agents on remote hosts for easy, secure connectivity
- **Simple UI-based setup**: Add remote hosts by just entering IP/URL and token
- **Automatic scanning**: Configurable periodic scans (default: every 5 minutes)
- **Historical tracking**: All container states are timestamped and stored
- **Web UI**: Clean, responsive web interface with real-time updates
- **REST API**: Full API access to all container and host data
- **Container management**: Start, stop, restart, remove containers, and view logs
- **Image management**: List, remove, and prune images across all hosts
- **Single container deployment**: Everything runs in one lightweight container
- **Multiple connection types**: Agent (recommended), Unix socket, TCP, and SSH connections
- **Anonymous telemetry** (optional): Track container usage trends with privacy-first design

## Quick Start

### Using Docker Compose (Recommended)

The easiest way to get started:

# 1. Grab docker-compose.yml
Create .env file with Docker socket GID
echo "DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)" >> .env

# 2. Create and modify the config file (optional)
`cp config/config.yaml.example config/config.yaml`

# 3. Start the server
`docker-compose up -d`

# 4. Access the UI
`open http://localhost:8080`

## Configuration

### Option 1: Agent-Based Hosts (Recommended)

The easiest way to add remote hosts is using the **lightweight agent**:

1. **Deploy agent on remote host:**
Grab docker-compose.agent.example.yml

2. **Get the API token from logs:**

   ```bash
   docker logs census-agent | grep "API Token"
   ```

3. **Add in the UI of the server:**
   - Click **"+ Add Agent Host"** button
   - Enter host name, agent URL (`http://host-ip:9876`), and token
   - Click **"Test Connection"** then **"Add Agent"**

---

### Connection Types

- **Agent** (Recommended): `agent://hostname:9876` or `http://hostname:9876` - Lightweight agent with token auth
- **Unix Socket**: `unix:///var/run/docker.sock` (local Docker daemon)

**Why use agents?** No SSH keys, no TLS certificates, no Docker daemon exposure - just a simple URL and token!

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

## Development

### Building Images

Use the interactive `build-all-images.sh` script in the scripts folder.

### Project Structure

- `cmd/server/main.go` - Server application entry point
- `cmd/agent/main.go` - Agent application entry point
- `internal/scanner/` - Docker host connection and container discovery
- `internal/agent/` - Agent server implementation
- `internal/storage/` - SQLite database operations with full CRUD
- `internal/api/` - HTTP handlers and routing
- `internal/models/` - Data structures shared across packages
- `web/` - Static frontend files served by the Go application
- `scripts/` - Utility scripts for building and deployment

### Adding New Features

1. Update models in `internal/models/models.go`
2. Add database operations in `internal/storage/db.go`
3. Implement API handlers in `internal/api/handlers.go`
4. Update frontend in `web/` directory

## Telemetry & Analytics

Container Census includes an optional telemetry system to track anonymous container usage statistics. This helps understand trends and allows you to monitor your own infrastructure.

### Key Features

- 📊 **Anonymous data collection** - No personal information collected
- 🔄 **Multi-endpoint support** - Send to public and/or private analytics servers
- 🏢 **Self-hosted analytics** - Run your own telemetry collector
- 📈 **Visual dashboards** - Charts showing popular images, growth trends
- 🔒 **Opt-in by default** - Disabled unless explicitly enabled
- 🌐 **Server aggregation** - Server collects stats from all agents before submission

### Quick Start

Enable in `config/config.yaml`:

```yaml
telemetry:
  enabled: true
  interval_hours: 168  # Weekly
  endpoints:
    - name: public
      url: https://telemetry.container-census.io/api/ingest
      enabled: true
    - name: private
      url: http://my-analytics:8081/api/ingest
      enabled: true
      api_key: "your-key"
```

### Run Your Own Analytics Server

```bash
# Start telemetry collector with dashboard
docker-compose -f docker-compose.telemetry.yml up -d

# Access dashboard
open http://localhost:8081
```

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
