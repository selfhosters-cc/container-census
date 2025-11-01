# Local Testing Instructions

This guide covers how to build, test, and run Container Census locally for development and testing.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Setting Up Go](#setting-up-go)
- [Building the Project](#building-the-project)
- [Running Tests](#running-tests)
- [Running Locally](#running-locally)
- [Common Issues](#common-issues)

## Prerequisites

### Required Tools
- **Go 1.23+** with CGO enabled (required for SQLite)
- **Docker** (for scanning containers)
- **Make** (optional, but recommended)

### Check If Go Is Installed
```bash
go version
```

If you see `command not found`, proceed to [Setting Up Go](#setting-up-go).

## Setting Up Go

### Installation

**Ubuntu/Debian:**
```bash
# Download and install Go 1.23
cd /tmp
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
```

**macOS:**
```bash
brew install go
```

**Manual Download:**
Visit https://go.dev/dl/ and download the appropriate version for your system.

### Add Go to Your PATH

**Option 1: Current Terminal Session Only**
```bash
export PATH=$PATH:/usr/local/go/bin
export GOTOOLCHAIN=auto
```

**Option 2: Permanent (Recommended)**

Add these lines to your shell profile (`~/.bashrc`, `~/.zshrc`, or `~/.profile`):

```bash
# Go environment
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
export GOTOOLCHAIN=auto
```

Then reload your shell:
```bash
source ~/.bashrc  # or ~/.zshrc
```

### Verify Go Installation

```bash
go version
```

Expected output:
```
go version go1.23.0 linux/amd64
```

## Building the Project

### Using Make (Recommended)

```bash
# Build all components
make build

# Build specific components
make build-server
make build-agent
make build-telemetry
```

Built binaries will be in `./bin/`:
- `./bin/census-server`
- `./bin/census-agent`
- `./bin/telemetry-collector`

### Manual Build (Without Make)

#### Build Server
```bash
export PATH=$PATH:/usr/local/go/bin
export GOTOOLCHAIN=auto
CGO_ENABLED=1 go build -o ./bin/census-server ./cmd/server
```

#### Build Agent
```bash
CGO_ENABLED=1 go build -o ./bin/census-agent ./cmd/agent
```

#### Build Telemetry Collector
```bash
CGO_ENABLED=1 go build -o ./bin/telemetry-collector ./cmd/telemetry-collector
```

### Build to Custom Location

```bash
# Build to /tmp for testing
CGO_ENABLED=1 go build -o /tmp/census-server ./cmd/server
```

### Verify Build

```bash
./bin/census-server --version
```

Expected output:
```
Container Census Server v1.3.23
```

## Running Tests

### Run All Tests

```bash
make test
```

Or manually:
```bash
CGO_ENABLED=1 go test -v ./...
```

### Run Specific Package Tests

```bash
# Test storage package
CGO_ENABLED=1 go test -v ./internal/storage

# Test notifications package
CGO_ENABLED=1 go test -v ./internal/notifications

# Test auth package
CGO_ENABLED=1 go test -v ./internal/auth
```

### Run Tests with Coverage

```bash
CGO_ENABLED=1 go test -v -cover ./...
```

### Run Tests with Race Detection

```bash
CGO_ENABLED=1 go test -v -race ./...
```

### Run Specific Test

```bash
# Run a specific test function
CGO_ENABLED=1 go test -v ./internal/storage -run TestGetChangesReport

# Run tests matching a pattern
CGO_ENABLED=1 go test -v ./internal/storage -run "TestGetChangesReport.*"
```

## Running Locally

### Quick Start

**1. Create Configuration File**

```bash
# Copy example config
cp config/config.example.yaml config/config.yaml

# Edit with your settings
nano config/config.yaml
```

**2. Build and Run**

```bash
make dev
```

Or manually:
```bash
CGO_ENABLED=1 go build -o ./bin/census-server ./cmd/server
./bin/census-server
```

Server will start on **http://localhost:8080** (default port).

### Run on Custom Port

#### Option 1: Environment Variable

```bash
export SERVER_PORT=3000
./bin/census-server
```

Server will start on **http://localhost:3000**

#### Option 2: Config File

Edit `config/config.yaml`:
```yaml
server:
  port: 3000
```

**Note:** Command line flags are not supported. Use environment variables or config file.

### Run with Authentication Disabled (Development)

```bash
export AUTH_ENABLED=false
./bin/census-server
```

### Run with Custom Database Location

```bash
export DB_PATH=/tmp/census-test.db
./bin/census-server
```

### Run with Debug Logging

```bash
export LOG_LEVEL=debug
./bin/census-server
```

### Full Development Setup Example

```bash
# Set environment
export PATH=$PATH:/usr/local/go/bin
export GOTOOLCHAIN=auto
export SERVER_PORT=3000
export AUTH_ENABLED=false
export DATABASE_PATH=/tmp/census-dev.db
export LOG_LEVEL=debug

# Build
CGO_ENABLED=1 go build -o /tmp/census-server ./cmd/server

# Run
/tmp/census-server
```

Output:
```
2025-10-31 15:00:00 INFO  Starting Container Census Server v1.3.23
2025-10-31 15:00:00 INFO  Authentication: disabled
2025-10-31 15:00:00 INFO  Database: /tmp/census-dev.db
2025-10-31 15:00:00 INFO  Server listening on :3000
2025-10-31 15:00:00 INFO  Web UI: http://localhost:3000
```

### Access the UI

Open your browser:
```
http://localhost:3000
```

### Scan Local Docker Containers

The server will automatically scan the local Docker socket if you have Docker running and the socket is accessible at `/var/run/docker.sock`.

To verify Docker access:
```bash
docker ps
```

If you see permission errors, you may need to add your user to the docker group:
```bash
sudo usermod -aG docker $USER
newgrp docker
```

## Running the Agent Locally

### Build Agent

```bash
CGO_ENABLED=1 go build -o ./bin/census-agent ./cmd/agent
```

### Run Agent on Custom Port

```bash
export API_TOKEN=test-token-123
./bin/census-agent -port 9876
```

Or use the default port (9876) by omitting the flag.

### Test Agent Connection

```bash
curl -H "X-API-Token: test-token-123" http://localhost:9876/health
```

Expected response:
```json
{
  "status": "healthy",
  "version": "1.3.23"
}
```

## Running the Telemetry Collector Locally

### Prerequisites

Telemetry collector requires PostgreSQL.

**Start PostgreSQL with Docker:**
```bash
docker run -d \
  --name census-postgres \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=telemetry \
  -p 5432:5432 \
  postgres:15
```

### Build and Run Collector

```bash
# Build
CGO_ENABLED=1 go build -o ./bin/telemetry-collector ./cmd/telemetry-collector

# Set database URL
export DATABASE_URL="postgres://postgres:password@localhost:5432/telemetry?sslmode=disable"
export PORT=8081

# Run
./bin/telemetry-collector
```

### Test Collector

```bash
curl http://localhost:8081/health
```

## Common Issues

### Issue: `go: command not found`

**Solution:**
Go is not in your PATH. Add it:
```bash
export PATH=$PATH:/usr/local/go/bin
```

### Issue: `gcc: command not found` or CGO errors

**Solution:**
SQLite requires CGO and a C compiler.

**Ubuntu/Debian:**
```bash
sudo apt-get install build-essential
```

**macOS:**
```bash
xcode-select --install
```

### Issue: `cannot find package "github.com/mattn/go-sqlite3"`

**Solution:**
Dependencies not installed. Run:
```bash
go mod download
go mod tidy
```

### Issue: Permission denied accessing Docker socket

**Solution:**
Add your user to the docker group:
```bash
sudo usermod -aG docker $USER
newgrp docker
```

Or run with sudo (not recommended for development):
```bash
sudo ./bin/census-server
```

### Issue: Port already in use

**Solution:**
Change the port:
```bash
export SERVER_PORT=3001
./bin/census-server
```

Or kill the process using the port:
```bash
# Find process
lsof -i :8080

# Kill it
kill -9 <PID>
```

### Issue: Database locked

**Solution:**
Another instance is running or database is corrupted.

```bash
# Stop other instances
pkill census-server

# Delete test database
rm /tmp/census-dev.db

# Restart
./bin/census-server
```

### Issue: Tests fail with "unsupported platform"

**Solution:**
Ensure CGO is enabled:
```bash
export CGO_ENABLED=1
go test -v ./...
```

## Quick Reference

### Build Commands
```bash
# Server
CGO_ENABLED=1 go build -o ./bin/census-server ./cmd/server

# Agent
CGO_ENABLED=1 go build -o ./bin/census-agent ./cmd/agent

# Telemetry Collector
CGO_ENABLED=1 go build -o ./bin/telemetry-collector ./cmd/telemetry-collector
```

### Test Commands
```bash
# All tests
CGO_ENABLED=1 go test -v ./...

# Specific package
CGO_ENABLED=1 go test -v ./internal/storage

# With coverage
CGO_ENABLED=1 go test -v -cover ./...
```

### Run Commands
```bash
# Default (port 8080)
./bin/census-server

# Custom port
SERVER_PORT=3000 ./bin/census-server

# No auth
AUTH_ENABLED=false ./bin/census-server

# Custom DB
DATABASE_PATH=/tmp/test.db ./bin/census-server
```

## Development Workflow

### Typical Development Cycle

```bash
# 1. Make code changes
nano internal/storage/db.go

# 2. Run tests
CGO_ENABLED=1 go test -v ./internal/storage

# 3. Build
CGO_ENABLED=1 go build -o /tmp/census-server ./cmd/server

# 4. Run locally
SERVER_PORT=3000 AUTH_ENABLED=false /tmp/census-server

# 5. Test in browser
open http://localhost:3000

# 6. Check logs
tail -f /var/log/census-server.log
```

### Using Make for Development

```bash
# Format code
make fmt

# Lint code
make lint

# Run tests
make test

# Build and run
make dev
```

## Environment Variables Reference

### Server
- `SERVER_PORT` - HTTP server port (default: 8080)
- `SERVER_HOST` - HTTP server host (default: 0.0.0.0)
- `DATABASE_PATH` - SQLite database path (default: ./data/census.db)
- `CONFIG_PATH` - Config file path (default: ./config/config.yaml)
- `AUTH_ENABLED` - Enable authentication (default: true)
- `AUTH_USERNAME` - Basic auth username
- `AUTH_PASSWORD` - Basic auth password
- `LOG_LEVEL` - Logging level (debug/info/warn/error)
- `SCANNER_INTERVAL_SECONDS` - Scan interval in seconds (default: 300)
- `TELEMETRY_INTERVAL_HOURS` - Telemetry reporting interval (default: 168)
- `TZ` - Timezone for telemetry (default: UTC)

**Note:** Server does not support command-line flags. Use environment variables or config file.

### Agent
- `API_TOKEN` - Authentication token (required)
- `-port` flag - HTTP server port (default: 9876)
- `-token` flag - Alternative way to specify API token

**Note:** Agent supports command-line flags: `./bin/census-agent -port 9876 -token your-token`

### Telemetry Collector
- `DATABASE_URL` - PostgreSQL connection string (required)
- `PORT` - HTTP server port (default: 8081)
- `COLLECTOR_AUTH_ENABLED` - Protect dashboard UI (default: false)
- `COLLECTOR_AUTH_USERNAME` - Basic auth username
- `COLLECTOR_AUTH_PASSWORD` - Basic auth password

**Note:** Telemetry collector uses `PORT` (not `SERVER_PORT`).

## Next Steps

- Read [CLAUDE.md](CLAUDE.md) for architecture details
- Check [README.md](README.md) for deployment options
- See [Makefile](Makefile) for all available commands
- Review tests in `internal/*/` directories for examples

## Getting Help

If you encounter issues not covered here:

1. Check existing GitHub issues: https://github.com/selfhosters-cc/container-census/issues
2. Review logs: `./bin/census-server` outputs logs to stdout
3. Enable debug logging: `export LOG_LEVEL=debug`
4. Run tests to verify environment: `make test`

For questions or bug reports, please open an issue on GitHub.
