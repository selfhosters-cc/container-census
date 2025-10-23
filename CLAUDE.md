# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Container Census is a multi-host Docker monitoring system written in Go. It consists of three main applications:

1. **Server** (`cmd/server`): Main monitoring application with web UI and REST API
2. **Agent** (`cmd/agent`): Lightweight agent for remote Docker hosts
3. **Telemetry Collector** (`cmd/telemetry-collector`): Analytics aggregation service with PostgreSQL backend

## Build and Development Commands

### Prerequisites
- Go 1.21+ with CGO enabled (required for SQLite)
- Docker socket GID must be determined: `stat -c '%g' /var/run/docker.sock`

### Local Development
```bash
# Setup
make setup                  # Install deps + create config from example

# Build and run locally (requires CGO_ENABLED=1)
make build                  # Build server binary
make run                    # Run server
make dev                    # Build + run

# Code quality
make fmt                    # Format code
make lint                   # Vet code
make test                   # Run tests
```

### Docker Development
```bash
# Single container
make docker-build           # Build with auto-detected Docker GID
make docker-run            # Build + run container
make docker-stop           # Stop and remove

# Docker Compose (recommended)
make compose-up            # Build + start all services
make compose-down          # Stop all services
make compose-logs          # Follow logs

# Manual docker-compose
DOCKER_GID=$(stat -c '%g' /var/run/docker.sock) docker-compose up -d
```

### Building Container Images
```bash
# Interactive script (easiest - recommended)
./scripts/build-all-images.sh

# Manual builds using docker buildx (for multi-architecture support)
# Single platform (local use):
docker buildx build --platform linux/amd64 --build-arg DOCKER_GID=999 -t container-census:latest --load .
docker buildx build --platform linux/amd64 --build-arg DOCKER_GID=999 -f Dockerfile.agent -t census-agent:latest --load .
docker buildx build --platform linux/amd64 -f Dockerfile.telemetry-collector -t telemetry-collector:latest --load .

# Multi-platform (requires push to registry):
docker buildx build --platform linux/amd64,linux/arm64 --build-arg DOCKER_GID=999 -t container-census:latest --push .

# Legacy docker build (amd64 only, no buildx):
docker build --build-arg DOCKER_GID=$(stat -c '%g' /var/run/docker.sock) -t container-census:latest .
docker build --build-arg DOCKER_GID=$(stat -c '%g' /var/run/docker.sock) -f Dockerfile.agent -t census-agent:latest .
docker build -f Dockerfile.telemetry-collector -t telemetry-collector:latest .
```

## Architecture

### Three-Tier System Design

**Census Server** (main application):
- SQLite database for container history
- Periodic scanner that queries all configured hosts
- REST API for management operations
- Static web UI (vanilla JavaScript)
- Optional telemetry submission to collector(s)

**Census Agent** (deployed to remote hosts):
- Stateless HTTP API wrapper around Docker socket
- Token-based authentication
- No database - just proxies Docker API calls
- Single binary, runs in ~10MB container

**Telemetry Collector** (analytics aggregation):
- PostgreSQL database for aggregate statistics
- Public ingestion API (no auth required)
- Optional Basic Auth for dashboard UI
- Aggregates data from multiple census-server installations

### Key Architectural Patterns

#### Host Connection Types
The scanner (`internal/scanner/scanner.go`) supports multiple connection methods:
- `unix://` - Local Docker socket
- `agent://` or `http://` - Agent-based (recommended for remote hosts)
- `tcp://` - Direct Docker API (requires TLS setup)
- `ssh://` - SSH tunneling (requires key auth)

Connection type is auto-detected from address prefix in `cmd/server/main.go:detectHostType()`.

#### Authentication Architecture
**Census Server** (`internal/auth/middleware.go`):
- Basic Auth protects **all** `/api/*` endpoints (management operations)
- Basic Auth protects static UI files
- Only `/api/health` is public

**Telemetry Collector** (`cmd/telemetry-collector/main.go`):
- `/api/ingest` is **always public** (anonymous telemetry)
- `/api/stats/*` endpoints are **always public** (read-only)
- Basic Auth **only** protects static dashboard files when `COLLECTOR_AUTH_ENABLED=true`

#### Database Deduplication Strategy
Telemetry collector uses 7-day deduplication windows:
- If installation submits within 7 days → **UPDATE** existing record
- If installation submits after 7+ days → **INSERT** new record
- Charts use `DISTINCT ON (installation_id)` to show only latest data per installation
- Total submissions count reflects actual DB records (not API calls)

Implementation: `cmd/telemetry-collector/main.go:saveTelemetry()`

#### Telemetry Collection Flow
1. **Server aggregates**: `internal/telemetry/collector.go` gathers data from all agents/hosts
2. **Server submits**: `internal/telemetry/submitter.go` sends to configured endpoints
3. **Collector receives**: `cmd/telemetry-collector/main.go:handleIngest()`
4. **Collector deduplicates**: Updates existing or inserts new based on 7-day window
5. **Dashboard queries**: Uses `DISTINCT ON` to show latest per installation

Server reads `TZ` environment variable and includes timezone in reports for privacy-friendly geographic distribution.

### Package Structure

```
internal/
├── agent/          # Agent server implementation (HTTP wrapper for Docker)
├── api/            # REST API handlers for census server
├── auth/           # HTTP Basic Auth middleware
├── config/         # YAML configuration loading
├── models/         # Shared data structures across all apps
├── scanner/        # Multi-protocol Docker scanning (unix/agent/tcp/ssh)
├── storage/        # SQLite operations for census server
├── telemetry/      # Telemetry collection, scheduling, submission
└── version/        # Version string from .version file

cmd/
├── server/                # Census server main application
├── agent/                 # Lightweight agent for remote hosts
└── telemetry-collector/   # PostgreSQL-backed analytics service

web/                # Static files for census server UI
web/analytics/      # Static files for telemetry dashboard
```

## Configuration

### Census Server
Uses `config/config.yaml` with environment variable overrides:
- `CONFIG_PATH` - Path to config file
- `AUTH_ENABLED` - Enable/disable authentication
- `AUTH_USERNAME` / `AUTH_PASSWORD` - Credentials
- `TZ` - Timezone for telemetry (e.g., `America/Toronto`)

Hosts can be configured in YAML or added via UI. Database takes precedence.

### Telemetry Collector
Environment-only configuration:
- `DATABASE_URL` - PostgreSQL connection string
- `PORT` - Listen port (default 8081)
- `COLLECTOR_AUTH_ENABLED` - Protect dashboard UI only
- `COLLECTOR_AUTH_USERNAME` / `COLLECTOR_AUTH_PASSWORD`

### Agent
Environment-only configuration:
- `PORT` - Listen port (default 9876)
- `API_TOKEN` - Auto-generated on first start, logged to stdout

## Common Development Patterns

### Adding New API Endpoints

1. Define request/response structs in `internal/models/models.go`
2. Add database methods to `internal/storage/db.go` (for server) or SQL queries (for collector)
3. Implement handler in `internal/api/handlers.go` (server) or `cmd/telemetry-collector/main.go` (collector)
4. Register route in `setupRoutes()` with appropriate auth middleware
5. Update frontend JavaScript in `web/app.js` or `web/analytics/app.js`

### Adding Telemetry Metrics

To track new metrics in telemetry:

1. Extend `Container` model in `internal/models/models.go` with new fields
2. Update `TelemetryReport` model to aggregate the data
3. Modify `internal/scanner/scanner.go` to collect raw data from Docker
4. Update `internal/telemetry/collector.go:CollectReport()` to aggregate
5. Add database columns in `cmd/telemetry-collector/main.go:initSchema()`
6. Update INSERT/UPDATE queries in `saveTelemetry()`
7. Create API endpoint and chart in `web/analytics/`

**IMPORTANT - Backward Compatibility:**
- When removing fields from API responses, ensure the telemetry collector's database queries handle missing columns gracefully
- Use SQL's `COALESCE()` or conditional logic to provide defaults for missing fields
- API endpoints should not break if older data lacks certain fields
- Frontend code should handle `null`/`undefined` values for fields that may not exist in all records
- Keep database columns even if not displayed in UI - they may be re-added later or used by older versions
- Example: `image_stats.size_bytes` column exists in DB but is not returned by `/api/stats/image-details` endpoint
  - Query selects only `count`, not `size_bytes`
  - Old telemetry submissions with `size_bytes` continue to work
  - New submissions can omit it or include it (ignored)
  - Column remains in schema for potential future use

### UI Refresh Pattern

The web UI maintains local state and doesn't automatically refresh after mutations. When implementing delete/update operations:

```javascript
async function deleteResource(id) {
    await fetch(`/api/resource/${id}`, { method: 'DELETE' });
    await loadData();  // Refresh all data

    // If on specific tab, also re-render that view
    if (currentTab === 'resources') {
        renderResources(resources);
    }
}
```

See recent fix in `web/app.js:loadData()` for host deletion refresh pattern.

### Version Management

Version is stored in `.version` file at repository root and embedded at build time:
- Server/Agent: `internal/version/version.go` reads from `.version`
- Docker: `docker-entrypoint.sh` copies `.version` to `/.version` in container
- Telemetry: Version included in all reports for distribution tracking

Update `.version` before building/tagging releases.

### Version Update Notifications

Container Census automatically checks for updates and notifies users through multiple channels:

**GitHub Release Requirement:**
- All releases MUST be created as GitHub Releases on `selfhosters-cc/container-census`
- The build script (`scripts/build-all-images.sh`) prompts to create releases after pushing images
- Releases use tag format: `v{VERSION}` (e.g., `v0.9.23`)
- Release notes are auto-generated using `gh release create --generate-notes`

**Version Checking Architecture:**
- **Backend**: `internal/version/version.go` contains `CheckLatestVersion()` function
  - Queries GitHub Releases API: `https://api.github.com/repos/selfhosters-cc/container-census/releases/latest`
  - Results cached for 24 hours to respect rate limits (60 requests/hour unauthenticated)
  - Thread-safe with RWMutex for concurrent access
  - Semantic version comparison (major.minor.patch)
  - Returns `UpdateInfo` struct with current version, latest version, availability flag, and release URL

**Health Endpoint Integration:**
- `/api/health` endpoint includes version information:
  ```json
  {
    "status": "healthy",
    "version": "0.9.22",
    "latest_version": "0.9.23",
    "update_available": true,
    "release_url": "https://github.com/selfhosters-cc/container-census/releases/tag/v0.9.23"
  }
  ```
- Available in both census server and telemetry collector

**UI Notification:**
- Version badge in header shows update arrow when available: `v0.9.22 → v0.9.23 ⬆️`
- Badge is clickable and opens release page in new tab
- Implemented in both vanilla JS dashboards (`web/app.js`, `web/analytics/app.js`)
- Console log message with download link

**Server Log Notification:**
- All three applications (server, agent, collector) check for updates:
  - On startup (asynchronous, non-blocking)
  - Daily at midnight (background goroutine)
- Log format:
  ```
  ⚠️  UPDATE AVAILABLE: Container Census v0.9.22 → v0.9.23
     Download: https://github.com/selfhosters-cc/container-census/releases/tag/v0.9.23
  ```

**Implementation Details:**
- Startup check: `go checkForUpdates()` launched before HTTP server starts
- Daily check: `go runDailyVersionCheck(ctx)` runs with 24-hour ticker
- Both functions are non-blocking and handle errors gracefully
- "dev" builds do not show update notifications
- Version check failures are logged but do not affect application operation

**Rate Limiting Considerations:**
- GitHub API unauthenticated limit: 60 requests/hour
- With 24-hour cache per instance, supports ~1,440 installations checking concurrently
- Cache invalidation available via `version.InvalidateCache()` if needed
- No authentication required (public repository, public releases)

## Database Schemas

### Census Server (SQLite)
- `hosts` - Configured Docker hosts
- `containers` - Historical container records (timestamped)
- `images` - Image data per host
- `scan_results` - Scan execution history

### Telemetry Collector (PostgreSQL)
- `telemetry_reports` - Aggregate statistics per installation (7-day deduplication)
- `image_stats` - Per-image usage counts and sizes

Both support schema migrations via `IF NOT EXISTS` and `ALTER TABLE IF NOT EXISTS`.

## Docker Socket Permissions

The server container runs as non-root (UID 1000) but needs Docker socket access. Solution:

1. Build-time: `--build-arg DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)`
2. Runtime: Container user is added to this GID via `docker-entrypoint.sh`
3. Socket mount: `-v /var/run/docker.sock:/var/run/docker.sock`

The `group_add` approach in docker-compose.yml is more portable than build-arg.

## Testing

Currently minimal test coverage. To run existing tests:
```bash
make test
# or
go test -v ./...
```

When adding tests, ensure CGO is enabled for SQLite tests.

## Important Implementation Notes

- All date/time operations use UTC internally
- Image names are normalized (registry prefixes removed) for aggregation
- Scanner timeout (30s default) applies per-host
- Agent tokens are logged only once on first startup
- Telemetry submissions include retry logic (3 attempts with exponential backoff)
- Web UI auto-refreshes every 30 seconds
- Chart.js 4.4.0 is used for all data visualizations
