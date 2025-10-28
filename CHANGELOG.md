# Changelog

All notable changes to Container Census will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
- **Enhanced container lifecycle timeline display**:
  - First detected events now show initial state: "Container 'name' first detected (running)" or "(stopped)"
  - Added "last seen" event showing most recent observation and total scan count
  - Added comprehensive summary banner with statistics (total observations, state changes, image updates, current status)
  - Timeline now clearly shows container activity over time
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
