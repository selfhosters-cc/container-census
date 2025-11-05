# Changelog

All notable changes to Container Census will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.5.0] - 2025-11-04

### Changed

- **Database-First Configuration**: Complete migration from YAML config to database storage
  - All application settings now stored in SQLite database
  - Config file only used for one-time migration on first run
  - Settings managed through Web UI and API
  - Environment variables for deployment-specific settings only (AUTH_*, SERVER_*, DATABASE_PATH)
  - Export/Import functionality for backup and migration between instances

- **Improved Fresh Install Experience**
  - Auto-creates local Unix socket host on first run
  - Interactive onboarding tour for new users
  - Dashboard as default landing page
  - Security scanning defaults to disabled (opt-in)
  - Community telemetry defaults to disabled (opt-in)

- **Enhanced Dashboard**
  - Functional telemetry toggle - enable/disable directly from dashboard
  - Security toggle downloads Trivy DB when enabled
  - Recent Activity section shows scan/telemetry events


## [1.3.0] - 2025-10-30

### Added

- **CPU & Memory Monitoring**: Comprehensive resource usage tracking with historical trends
  - Real-time CPU and memory stats collected during each scan
  - Two-tier data retention: granular data (1 hour) + hourly aggregates (2 weeks)
  - Per-host stats collection toggle - enable/disable for each host individually
  - Interactive time-series charts with time range selection (1h, 24h, 7d, all)
  - Mini sparkline charts in monitoring grid for quick trend visualization
  - Dual-axis charts showing CPU % and Memory MB on separate scales
  - Stats summary showing average, min, and max values
  - Works with all connection types: local socket, agents, TCP, and SSH
  - Background job for automatic hourly aggregation and cleanup
  - Database schema with `container_stats_aggregates` table for long-term storage

- **Prometheus Metrics Endpoint - untested**: Export metrics for Grafana integration
  - `/api/metrics` endpoint in Prometheus text format
  - Exports `census_container_cpu_percent` and `census_container_memory_bytes`
  - Labels include container name, host name, and image
  - Compatible with Prometheus scraping and Grafana dashboards

- **Monitoring Tab**: New dedicated tab for resource monitoring
  - Grid layout showing all running containers with stats enabled
  - Real-time CPU and memory usage display
  - Mini sparkline charts showing recent trends (last 20 data points)
  - "View Detailed Stats" button opens modal with full time-series data
  - Filters to show only containers from enabled hosts

- **Stats Collection API Endpoints**:
  - `GET /api/containers/{host_id}/{container_id}/stats?range={1h|24h|7d|all}` - Historical stats
  - `GET /api/config` - Get current configuration including scanner interval
  - `POST /api/config/scanner` - Update scanner interval

### Changed

- **Agent Stats Collection**: Improved concurrent collection for better performance
  - Changed from sequential to parallel stats collection using goroutines
  - Prevents timeout issues when scanning hosts with many containers
  - Uses `sync.WaitGroup` for safe concurrent collection

- **Docker Stats Method**: Switched to streaming stats for accurate CPU measurement
  - Uses Docker API `stream=true` parameter to get two samples
  - Calculates CPU percentage from delta between samples
  - Fixes issue where single-shot stats always returned 0% CPU
  - Implemented in both scanner (local) and agent (remote) code paths

- **Database Storage**: Enhanced stats field handling
  - Removed `omitempty` JSON tags from stats fields to always include zero values
  - Uses `sql.NullFloat64` and `sql.NullInt64` for proper NULL handling
  - Stores all stats values including 0% CPU (previously skipped)
  - Added migration for new columns: `cpu_percent`, `memory_usage`, `memory_limit`, `memory_percent`

- **Host Management UI**: Added visual indicators for stats collection status
  - Clickable badges to toggle stats collection per host
  - "✓ Enabled" (green) and "Disabled" (gray) badges
  - Immediate visual feedback when toggling

### Fixed

- **Monitoring Tab Filtering**: Fixed containers not appearing from remote hosts
  - Changed filter from stats-based to state-based (all running containers)
  - Added enabled-host filtering to exclude disabled hosts
  - Stats-less containers now show placeholders instead of being hidden

- **Stats Modal Display**: Fixed modal not opening when clicking "View Detailed Stats"
  - Changed from `style.display = 'block'` to CSS class-based approach
  - Uses `.modal.show { display: flex !important; }` for proper display
  - Fixed button click handlers using programmatic event listeners

- **CPU Always Showing 0%**: Fixed streaming stats implementation
  - Root cause: Single-shot stats have zero/stale delta in `PreCPUStats`
  - Solution: Read two consecutive samples from streaming stats API
  - Properly calculates CPU delta across ~1 second interval

- **Database Storage of Zero Values**: Fixed 0% CPU not being saved
  - Changed storage condition from checking `CPUPercent > 0` to `MemoryLimit > 0`
  - Zero CPU values are valid data (idle containers) and now properly stored
  - Charts now display accurate data including idle periods

- **Agent Stats Timeout**: Fixed timeout errors with large container counts
  - Implemented concurrent collection instead of sequential
  - Reduced total collection time significantly
  - No longer hits 30-second scanner timeout with 20+ containers

## [1.2.2] - 2025-10-28

### Added

- **Agent Token via Environment Variable**: Agent now supports setting API token via `API_TOKEN` environment variable
  - Eliminates need to mount `/app/data` volume for token persistence
  - Token priority: (1) `--token` flag, (2) `API_TOKEN` env var, (3) file, (4) auto-generate
  - Simplifies deployment in environments where volume management is difficult
  - Particularly useful in Kubernetes/orchestrated environments

## [1.2.1] - 2025-10-28

### Changed

- **History Tab**: Timeline view now shows image version numbers (from -> to) when available

## [1.2.0] - 2025-10-28

### Added
  
- **New History tab**: View historical information about your containers in the census server UI
  - Shows when containers are first seen, started, stopped, image updates, and disappearances
  - Timeline view with detailed event history per container
  - Activity badges indicating state changes, image updates, and restart events
  - Smart disappearance detection (2-hour threshold to avoid false positives from scan gaps)
  - New API endpoints: `/api/containers/lifecycle` and `/api/containers/lifecycle/{host_id}/{container_id}`

- **Database Viewer**: Telemetry collector now includes a database inspection interface
  - View all PostgreSQL tables and their row counts
  - Inspect table schemas with column definitions and data types
  - Browse table data with pagination support
  - Access via `/database` route in the telemetry dashboard
  - Helpful for debugging and understanding collected telemetry data

- **Agent API Token Persistence**: Census agent now persists API tokens across restarts and upgrades
  - Tokens automatically saved to `/app/data/agent-token` on first generation
  - Tokens survive container restarts, upgrades, and host reboots
  - Mount volume at `/app/data` to enable persistence (recommended)
  - Clear logging: "Using existing API token" vs "Generated new API token"
  - File permissions: 0600 for security
  - Graceful fallback: warnings logged if volume not mounted

- **Test Connection Buttons**: Added test connection functionality for easier setup
  - Test telemetry collector endpoint before adding (validates URL and API key)
  - Test agent connection during setup (validates connectivity and authentication)
  - Clear success/failure messages help troubleshoot configuration issues
  - New API endpoint: `/api/telemetry/test-endpoint`

### Changed
- Improved agent token logging for easier retrieval (`grep "API Token:"` works for both new and existing tokens)
- Updated README.md with agent volume mount instructions for token persistence
- **Implemented automatic database cleanup routine**:
  - Removes redundant scan records older than 7 days while preserving important lifecycle events
  - Keeps first scan, last scan, state changes, image changes, and gap indicators
  - Runs daily to reduce database size by ~99% for stable containers
  - Logs cleanup statistics for visibility

### Fixed
- Fixed telemetry manual submission hanging issue by running submissions asynchronously with background context
  - Manual submissions no longer block the HTTP response
  - Extended timeout to 5 minutes for slow agent connections
  - Submission status logged to server logs
- Improved agent authentication error detection and reporting
  - Agent hosts now show "Auth Failed" status badge when API token is incorrect
  - Test connection button validates both connectivity and authentication
  - Clear error messages: "API token mismatch - please verify the token is correct"
  - Scan logs now include helpful authentication failure hints

## [1.1.0] - 2025-10-27

### Added
- Database view functionality to the telemetry collector
- Graph view showing container relationships via networks, dependencies, and links
- Table view of all containers
- Telemetry charts and visualizations

### Changed
- Updated telemetry charts and README documentation
- General code cleanup and refactoring

### Fixed
- Fixed bug in showing last scan date
- Fixed bug where removed images weren't accurately updated in the UI immediately
- Improved telemetry collector error handling

## [1.0.0] - 2025-XX-XX

### Added
- Initial release of Container Census
- Multi-host Docker monitoring with centralized dashboard
- Lightweight remote agents for Docker scanning
- Web UI with container management capabilities
- REST API for programmatic access
- Historical container tracking (no UI)
- Image management and pruning
- Optional community/private telemetry collection
- Support for Unix socket, Agent connections
- Basic authentication for secure access
- Automatic update notifications via GitHub Releases API

---

For installation and usage instructions, see [README.md](README.md).
