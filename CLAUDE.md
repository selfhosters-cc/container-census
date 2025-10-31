# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Container Census is a multi-host Docker monitoring system written in Go. It consists of three main applications:

1. **Server** (`cmd/server`): Main monitoring application with web UI and REST API
2. **Agent** (`cmd/agent`): Lightweight agent for remote Docker hosts
3. **Telemetry Collector** (`cmd/telemetry-collector`): Analytics aggregation service with PostgreSQL backend

## Build and Development Commands

### Prerequisites
- Go 1.23+ with CGO enabled (required for SQLite) and GOTOOLCHAIN=auto
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

**Interactive Build Script (Recommended)**
```bash
./scripts/build-all-images.sh
```

This interactive script (`scripts/build-all-images.sh`) provides a guided build experience:
- **Version Management**: Auto-increment (patch/minor/major), keep current, or custom
- **Selective Building**: Build server, agent, telemetry collector, or all
- **Multi-Architecture**: Single platform (linux/amd64 or linux/arm64) or both
- **Registry Push**: Optional push to Docker Hub, GHCR, or custom registry
- **GitHub Release**: Automated release creation via `gh` CLI
- **Compose Generation**: Creates docker-compose.yml with appropriate image tags

The script uses Docker buildx for multi-architecture support and handles:
- Builder creation/configuration
- Version embedding (reads/writes `.version` file)
- Single-platform builds with `--load` (images available locally)
- Multi-platform builds (cache only, requires push for local use)
- Registry authentication and pushing

**Manual Builds**
```bash
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

**Note on Multi-Platform Builds**: Multi-arch images built with `--platform linux/amd64,linux/arm64` are stored in buildx cache but won't appear in `docker images` until pushed to a registry and pulled back. Use single-platform builds with `--load` for immediate local availability.

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

**Census Agent** (`cmd/agent/main.go`):
- Token-based authentication for all `/api/*` endpoints via `X-API-Token` header
- Token source priority: (1) `--token` flag, (2) `API_TOKEN` env var, (3) persisted file, (4) auto-generate
- Auto-generates secure token on first startup using crypto/rand (32 bytes, hex-encoded)
- Persists token to `/app/data/agent-token` for survival across restarts/upgrades
- Token file created with 0600 permissions for security
- If `API_TOKEN` env var is set, uses that token and skips file persistence (no volume needed)
- If token file cannot be created (no volume mounted, no env var), logs warning and generates ephemeral token
- Public endpoints: `/health`, `/info` (no auth required)

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

#### CPU and Memory Monitoring Architecture

Container Census supports optional resource usage monitoring with trending capabilities, configurable per-host.

**Data Collection**:
- **Opt-in per host**: `CollectStats` boolean field on Host model (default: true)
- **Running containers only**: Stats collected only for containers in "running" state
- **Scanner integration**: `internal/scanner/scanner.go` calls Docker `ContainerStats()` API
- **Agent support**: Agent responds to `?stats=true` query parameter on `/api/containers` endpoint
- **All connection types**: Works with unix://, agent://, tcp://, and ssh:// connections

**Data Storage - Two-Tier Retention**:
- **Granular data** (last 1 hour): Full-resolution scans stored in `containers` table
  - Columns: `cpu_percent`, `memory_usage`, `memory_limit`, `memory_percent`
  - Collected at scan interval (default: once per minute, configurable)
- **Aggregated data** (1 hour - 2 weeks): Hourly averages in `container_stats_aggregates` table
  - Columns: `avg_cpu_percent`, `avg_memory_usage`, `max_cpu_percent`, `max_memory_usage`, `sample_count`
  - One row per container per hour
  - Unique constraint: `(container_id, host_id, timestamp_hour)`
- **Automatic aggregation**: Hourly job (`storage.AggregateOldStats()`) converts granular → aggregated
- **Cleanup**: Records older than 2 weeks are deleted

**API Endpoints**:
1. **`GET /api/containers`**: Returns latest container state including current CPU/memory stats
2. **`GET /api/containers/{hostId}/{containerId}/stats?range=1h|24h|7d|all`**: Time-series data
   - Automatically combines granular + aggregated data
   - Returns array of `ContainerStatsPoint` with timestamp, CPU%, memory usage/limit
3. **`GET /metrics`**: Prometheus-compatible metrics endpoint
   - Format: `census_container_cpu_percent`, `census_container_memory_bytes`, `census_container_memory_limit_bytes`
   - Labels: `container_name`, `container_id`, `host_name`, `image`
   - Only includes running containers with stats

**Frontend Visualization**:
- **Chart.js 4.4.0** used for all charts (matches analytics dashboard)
- **Containers table**: CPU/Memory columns with current values and inline sparklines (1-hour)
- **Stats modal**: Detailed CPU/memory line charts with time range selector (1h/24h/7d/All)
- **Monitoring tab**: Grid view of all running containers with trend charts
- **Auto-refresh**: 30-second refresh when modal is open

**Performance Considerations**:
- Stats collection adds ~100-200ms per running container to scan time
- Host-level opt-out via `CollectStats=false` disables collection entirely
- Scanner continues successfully even if stats collection fails for individual containers
- Errors logged but don't block scan completion

**Implementation Files**:
- Models: `internal/models/models.go` (Host.CollectStats, ContainerStatsPoint)
- Scanner: `internal/scanner/scanner.go` (stats collection logic)
- Agent: `internal/agent/agent.go` (stats query parameter support)
- Storage: `internal/storage/db.go` (schema, aggregation, queries)
- API: `internal/api/handlers.go` (stats and metrics endpoints)
- Frontend: `web/app.js`, `web/index.html` (charts and visualizations)

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
- `API_TOKEN` - API token for authentication. Priority order:
  1. Command-line flag `--token`
  2. Environment variable `API_TOKEN`
  3. Persisted token file at `/app/data/agent-token`
  4. Auto-generated (logged to stdout and saved to file if volume mounted)

### Notification System
Environment-only configuration:
- `NOTIFICATION_RATE_LIMIT_MAX` - Maximum notifications per hour (default: 100)
- `NOTIFICATION_RATE_LIMIT_BATCH_INTERVAL` - Batch interval in seconds when rate limited (default: 600)
- `NOTIFICATION_THRESHOLD_DURATION` - Duration threshold must be exceeded before alerting (default: 120 seconds)
- `NOTIFICATION_COOLDOWN_PERIOD` - Cooldown between alerts for same container (default: 300 seconds)

## Notification System Architecture

The notification system provides flexible event-based alerting through multiple channels (webhooks, ntfy, in-app) with sophisticated filtering, rate limiting, and anomaly detection.

### Core Components

**1. Notification Service** (`internal/notifications/notifier.go`):
- Main coordinator that processes events after each scan
- Detects lifecycle events (state changes, image updates)
- Monitors CPU/memory thresholds with duration requirements
- Detects anomalous behavior after image updates
- Matches events against rules with pattern filtering
- Enforces cooldowns and silences
- Rate-limits delivery with batching

**2. Channel Implementations** (`internal/notifications/channels/`):
- **Webhook**: HTTP POST with custom headers, 3-attempt retry
- **Ntfy**: Custom server support, Bearer auth, priority/tag mapping
- **In-App**: Writes to notification_log table for UI display

**3. Baseline Collector** (`internal/notifications/baseline.go`):
- Runs hourly to calculate 48-hour rolling averages
- Captures pre-update baselines for anomaly detection
- Stores per (container_id, host_id, image_id)

**4. Rate Limiter** (`internal/notifications/ratelimiter.go`):
- Token bucket algorithm (default: 100/hour)
- Batch queue with 10-minute summary notifications
- Per-channel batching to prevent notification storms

### Event Types

1. **new_image** - Image updated (tag or SHA changed)
2. **container_started** - Container transitioned to running
3. **container_stopped** - Container transitioned to exited
4. **container_paused** - Container paused
5. **container_resumed** - Container resumed from pause
6. **state_change** - Any other state transition
7. **high_cpu** - CPU usage > threshold for 120+ seconds
8. **high_memory** - Memory usage > threshold for 120+ seconds
9. **anomalous_behavior** - Post-update CPU/memory 25%+ higher than 48hr baseline

### Notification Rules

Rules match events using:
- **Event types**: Array of event types to match
- **Host filter**: Specific host ID or null for all hosts
- **Container pattern**: Glob pattern (e.g., `web-*`, `*-prod`)
- **Image pattern**: Glob pattern (e.g., `nginx:*`, `myapp:1.*`)
- **CPU threshold**: Percentage (e.g., 80.0) for high_cpu events
- **Memory threshold**: Percentage (e.g., 90.0) for high_memory events
- **Threshold duration**: Seconds threshold must be exceeded (default: 120)
- **Cooldown**: Seconds before re-alerting same container (default: 300)
- **Channels**: Array of channel IDs to send to

**Default Rules** (created on first startup):
1. "Container Stopped" → In-app notifications
2. "New Image Detected" → In-app notifications
3. "High Resource Usage" (CPU>80%, Memory>90%) → In-app notifications

### Silences

Mute notifications for:
- Specific host (by host_id)
- Specific container (by container_id + host_id)
- Pattern-based (container_pattern glob)
- Time-limited with expiry timestamp

### Database Schema

**notification_channels**: Channel configurations (type, config JSON, enabled)
**notification_rules**: Rules with event filters and thresholds
**notification_rule_channels**: Many-to-many rule→channel mapping
**notification_log**: Sent notifications with read/unread status
**notification_silences**: Active silences with expiry times
**container_baseline_stats**: 48hr rolling baselines for anomaly detection
**notification_threshold_state**: Tracks breach duration for threshold alerts

### API Endpoints

**Channels:**
- GET /api/notifications/channels - List all channels
- POST /api/notifications/channels - Create channel
- PUT /api/notifications/channels/{id} - Update channel
- DELETE /api/notifications/channels/{id} - Delete channel
- POST /api/notifications/channels/{id}/test - Test channel

**Rules:**
- GET /api/notifications/rules - List all rules
- POST /api/notifications/rules - Create rule
- PUT /api/notifications/rules/{id} - Update rule
- DELETE /api/notifications/rules/{id} - Delete rule

**Logs:**
- GET /api/notifications/log?limit=100&unread=true - Get notifications
- PUT /api/notifications/log/{id}/read - Mark as read
- POST /api/notifications/log/read-all - Mark all read
- DELETE /api/notifications/log/clear - Clear old (7 days OR beyond 100 most recent)

**Silences:**
- GET /api/notifications/silences - List active silences
- POST /api/notifications/silences - Create silence
- DELETE /api/notifications/silences/{id} - Delete silence

**Status:**
- GET /api/notifications/status - System stats (unread count, rules, channels, rate limit)

### Webhook Configuration Example

```json
{
  "name": "Discord Webhook",
  "type": "webhook",
  "enabled": true,
  "config": {
    "url": "https://discord.com/api/webhooks/...",
    "headers": {
      "Content-Type": "application/json"
    }
  }
}
```

### Ntfy Configuration Example

```json
{
  "name": "Ntfy Alerts",
  "type": "ntfy",
  "enabled": true,
  "config": {
    "server_url": "https://ntfy.example.com",
    "token": "tk_...",
    "topic": "container-alerts"
  }
}
```

### Anomaly Detection Flow

1. **Baseline Capture**: Hourly job calculates 48hr avg CPU/memory per container
2. **Image Update Detected**: Scanner detects image_id change via lifecycle events
3. **Post-Update Monitoring**: Next scans compare current stats against baseline
4. **Anomaly Trigger**: If current > baseline * 1.25, generate anomalous_behavior event
5. **Notification**: Rule matching fires if configured for anomaly events

### Rate Limiting & Batching

- **Token Bucket**: Refills to max every hour
- **Immediate Delivery**: If tokens available, send instantly
- **Queue When Limited**: Add to batch queue if no tokens
- **Batch Summary**: Every 10 minutes, send summary of queued notifications
- **Per-Channel Batching**: Groups by channel to minimize noise

### Implementation Files

- `internal/notifications/notifier.go` - Main service (600+ lines)
- `internal/notifications/ratelimiter.go` - Rate limiting
- `internal/notifications/baseline.go` - Baseline stats collector
- `internal/notifications/channels/*.go` - Channel implementations
- `internal/storage/notifications.go` - Database operations (550+ lines)
- `internal/storage/defaults.go` - Default rules initialization
- `internal/api/notifications.go` - REST API handlers (350+ lines)
- `cmd/server/main.go` - Integration and background jobs

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
