# Enhanced Telemetry Metrics Implementation

This document summarizes the enhanced metrics collection implementation for Container Census.

## Overview

The Container Census server and telemetry system have been upgraded to collect and report comprehensive container metrics including states, resource usage, restart statistics, image sizes, and timezone information.

## Changes Made

### 1. Data Models Extended

#### Container Model (`internal/models/models.go`)
Added the following fields to track additional metadata:
- `ImageSize int64` - Size of the container image in bytes
- `RestartCount int` - Number of times the container has restarted
- `CPUPercent float64` - Current CPU usage percentage (optional)
- `MemoryUsage int64` - Current memory usage in bytes (optional)
- `MemoryLimit int64` - Memory limit in bytes (optional)
- `MemoryPercent float64` - Memory usage percentage (optional)

#### TelemetryReport Model (`internal/models/models.go`)
Extended with comprehensive metrics:

**Container State Breakdown:**
- `ContainersRunning int` - Count of running containers
- `ContainersStopped int` - Count of stopped/exited containers
- `ContainersPaused int` - Count of paused containers
- `ContainersOther int` - Count of containers in other states

**Resource Usage Aggregates:**
- `AvgCPUPercent float64` - Average CPU usage across running containers
- `AvgMemoryBytes int64` - Average memory usage across running containers
- `TotalMemoryLimit int64` - Total memory limits across all containers

**Restart Statistics:**
- `AvgRestarts float64` - Average restart count per container
- `HighRestartContainers int` - Count of containers with >10 restarts

**Image Statistics:**
- `TotalImageSize int64` - Total size of all images in bytes
- `UniqueImages int` - Count of unique images

**System Information:**
- `Timezone string` - System timezone (e.g., "America/New_York")

#### ImageStat Model (`internal/models/models.go`)
- Added `SizeBytes int64` - Image size in bytes

### 2. Scanner Enhanced (`internal/scanner/scanner.go`)

The scanner now collects:
1. **Image sizes** - Retrieved from Docker image metadata
2. **Restart counts** - Retrieved via ContainerInspect API call
3. **Resource stats** - CPU and memory usage (commented out by default due to performance overhead)

**Note:** Resource stats collection is disabled by default because it requires additional API calls per container. To enable, uncomment the stats collection block in `scanner.go` lines 118-140.

### 3. Telemetry Collector Updated (`internal/telemetry/collector.go`)

The collector now:
- Aggregates container states (running/stopped/paused/other)
- Calculates average CPU and memory usage
- Computes restart statistics
- Sums image sizes
- Detects system timezone from `TZ` environment variable
- Counts unique images

### 4. Database Schema Updated (`cmd/telemetry-collector/main.go`)

#### New Columns in `telemetry_reports` table:
```sql
containers_running INTEGER DEFAULT 0
containers_stopped INTEGER DEFAULT 0
containers_paused INTEGER DEFAULT 0
containers_other INTEGER DEFAULT 0
avg_cpu_percent REAL DEFAULT 0.0
avg_memory_bytes BIGINT DEFAULT 0
total_memory_limit BIGINT DEFAULT 0
avg_restarts REAL DEFAULT 0.0
high_restart_containers INTEGER DEFAULT 0
total_image_size BIGINT DEFAULT 0
unique_images INTEGER DEFAULT 0
timezone VARCHAR(100)
```

#### New Column in `image_stats` table:
```sql
size_bytes BIGINT DEFAULT 0
```

**Migration Support:** The `initSchema()` function includes `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` statements to upgrade existing databases without data loss.

### 5. Telemetry Ingestion Updated

The `saveTelemetry()` function now:
- Persists all new metrics fields
- Saves image sizes in the `image_stats` table
- Handles both UPDATE (for 7-day window) and INSERT operations with new fields

## Usage

### Viewing Enhanced Metrics

The enhanced metrics are automatically collected and submitted with telemetry reports. No configuration changes are required.

### Enabling Resource Stats Collection (Optional)

To enable CPU and memory usage collection:

1. Edit `internal/scanner/scanner.go`
2. Uncomment lines 118-140 (the resource stats collection block)
3. Rebuild the server

**Warning:** Enabling resource stats adds overhead to container scans as it makes an additional API call per running container.

### Dashboard Visualization

The telemetry collector dashboard can now display:
- Container state distribution pie charts
- Resource usage trends over time
- Restart frequency analytics
- Image size trends
- Geographic distribution based on timezones

**See:** `TELEMETRY_ENHANCEMENTS.md` for details on implementing dashboard charts for these metrics.

## Data Flow

```
┌─────────────────────┐
│  Docker API         │
│  - List Containers  │
│  - List Images      │
│  - Inspect Container│
│  - (Stats - opt)    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Scanner            │
│  - Collects metadata│
│  - Image sizes      │
│  - Restart counts   │
│  - Resource stats   │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Container Model    │
│  - Full metadata    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Telemetry Collector│
│  - Aggregates stats │
│  - Calculates avgs  │
│  - Counts states    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Telemetry Report   │
│  - Enhanced metrics │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  PostgreSQL         │
│  - Stores data      │
│  - Provides analytics|
└─────────────────────┘
```

## Performance Considerations

1. **Image Size Collection:** Adds one ImageList API call per scan (minimal overhead)
2. **Restart Count Collection:** Adds one ContainerInspect call per container (moderate overhead)
3. **Resource Stats Collection:** Adds one ContainerStats call per running container (significant overhead - **disabled by default**)

## Privacy Considerations

All collected metrics are **anonymous**:
- No container names or IDs are transmitted
- No sensitive labels are sent
- Image names are transmitted but can be normalized/hashed if needed
- Timezone is coarse-grained (no precise location data)
- No IP addresses or hostnames are collected

## Example Telemetry Report

```json
{
  "installation_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "version": "0.6.0",
  "timestamp": "2025-10-16T20:00:00Z",
  "host_count": 3,
  "agent_count": 2,
  "total_containers": 25,
  "scan_interval_seconds": 300,
  "containers_running": 20,
  "containers_stopped": 4,
  "containers_paused": 1,
  "containers_other": 0,
  "avg_cpu_percent": 0.0,
  "avg_memory_bytes": 0,
  "total_memory_limit": 0,
  "avg_restarts": 0.48,
  "high_restart_containers": 2,
  "total_image_size": 5368709120,
  "unique_images": 15,
  "timezone": "America/New_York",
  "image_stats": [
    {
      "image": "nginx:latest",
      "count": 5,
      "size_bytes": 142000000
    },
    {
      "image": "postgres:15-alpine",
      "count": 3,
      "size_bytes": 238000000
    }
  ],
  "agent_versions": {
    "0.5.0": 1,
    "0.6.0": 1
  }
}
```

## Next Steps

1. **Implement Dashboard Charts** - See `TELEMETRY_ENHANCEMENTS.md` for chart implementation guide
2. **Enable Resource Stats (Optional)** - Uncomment scanner code if needed
3. **Add More Analytics** - Query the new fields for insights
4. **Monitor Performance** - Track scan duration with enhanced collection

## Files Modified

- `internal/models/models.go` - Extended data models
- `internal/scanner/scanner.go` - Enhanced container scanning
- `internal/telemetry/collector.go` - Aggregation logic
- `cmd/telemetry-collector/main.go` - Database schema and ingestion

## Backwards Compatibility

✅ The implementation is fully backwards compatible:
- Old telemetry reports (without new fields) are still accepted
- Database migration handles existing installations
- New fields default to zero/null if not provided
- No breaking API changes

## Testing

To test the enhanced metrics:

1. Start the server with the updated code
2. Perform a container scan
3. Trigger a telemetry submission
4. Check the telemetry collector logs
5. Query the PostgreSQL database to verify new fields are populated

```sql
SELECT
  installation_id,
  containers_running,
  containers_stopped,
  avg_restarts,
  total_image_size,
  timezone
FROM telemetry_reports
ORDER BY timestamp DESC
LIMIT 10;
```

## Support

For questions or issues related to enhanced metrics:
1. Check the logs for collection errors
2. Verify database schema migration completed
3. Ensure Docker API is accessible
4. Review `TELEMETRY_ENHANCEMENTS.md` for chart implementation

---

**Implementation Date:** 2025-10-16
**Version:** 0.6.0
**Status:** ✅ Complete and Tested
