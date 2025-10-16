# Telemetry Dashboard Enhancement Roadmap

This document outlines additional metrics and charts that could be added to the telemetry dashboard, along with the required data collection changes.

## Implemented Charts ✅

### 1. Activity Hours Heatmap
**Status:** ✅ Implemented
**Description:** Shows when telemetry reports are received by day of week and hour (UTC)
**Data Source:** `telemetry_reports.timestamp`
**Chart Type:** Bubble chart heatmap
**Purpose:** Understand global usage patterns and time zones

### 2. Scan Interval Distribution
**Status:** ✅ Implemented
**Description:** Shows how users configure their scan intervals
**Data Source:** `telemetry_reports.scan_interval`
**Chart Type:** Doughnut chart
**Purpose:** Understand typical configuration patterns

## Charts Requiring Enhanced Data Collection

### 1. Container State/Status Distribution
**Status:** ⚠️ Requires data collection enhancement
**Description:** Pie chart showing running vs stopped vs paused containers
**Current Limitation:** Telemetry currently only reports total container count
**Required Changes:**
- Add `ContainerStateStats` to `TelemetryReport`:
  ```go
  type ContainerStateStats struct {
      Running int `json:"running"`
      Stopped int `json:"stopped"`
      Paused  int `json:"paused"`
      Other   int `json:"other"`
  }
  ```
- Update telemetry collection in `internal/telemetry/collector.go` to aggregate container states
- Add database fields to `telemetry_reports` table
- Create API endpoint `/api/stats/container-states`
- Build doughnut chart visualization

**Benefit:** Shows actual usage vs dormant containers, helps understand deployment health

---

### 2. Resource Usage Trends
**Status:** ⚠️ Requires significant data collection
**Description:** Line chart showing average CPU/memory usage over time
**Current Limitation:** No resource metrics are currently collected
**Required Changes:**
- Add resource collection to agents and main server
- Extend `Container` model with resource fields:
  ```go
  type ResourceStats struct {
      CPUPercent    float64 `json:"cpu_percent"`
      MemoryUsage   int64   `json:"memory_usage"`    // bytes
      MemoryLimit   int64   `json:"memory_limit"`    // bytes
      MemoryPercent float64 `json:"memory_percent"`
  }
  ```
- Add to `TelemetryReport`:
  ```go
  type TelemetryReport struct {
      // ... existing fields
      AvgCPU    float64 `json:"avg_cpu_percent"`
      AvgMemory int64   `json:"avg_memory_bytes"`
      TotalMemoryLimit int64 `json:"total_memory_limit"`
  }
  ```
- Query Docker stats during scans (performance consideration!)
- Store time-series data in telemetry database
- Create API endpoint `/api/stats/resource-trends`
- Build dual-axis line chart

**Benefit:** Shows infrastructure health and growth patterns
**Performance Note:** Collecting stats adds overhead to scans

---

### 7. Geographic Distribution
**Status:** ⚠️ Privacy-sensitive, optional
**Description:** World map or region breakdown showing where software is used
**Current Limitation:** No geographic data collected (by design for privacy)
**Required Changes:**
- **Option 1 - Timezone-based (privacy-friendly):**
  - Collect system timezone from each installation
  - Approximate region from timezone
  - No IP geolocation needed

- **Option 2 - User-specified:**
  - Add optional `region` config field
  - Users voluntarily specify their region/country

- Add to `TelemetryReport`:
  ```go
  type TelemetryReport struct {
      // ... existing fields
      Timezone string `json:"timezone,omitempty"` // e.g., "America/New_York"
      Region   string `json:"region,omitempty"`   // e.g., "US-East", "EU-West"
  }
  ```
- Create API endpoint `/api/stats/geographic`
- Build world map or regional bar chart

**Benefit:** Understand global adoption
**Privacy Consideration:** Keep data coarse-grained (regions, not cities)

---

### 9. Image Size Trends
**Status:** ⚠️ Requires enhanced image metadata
**Description:** Shows average container image sizes over time
**Current Limitation:** Image sizes not currently collected
**Required Changes:**
- Extend image scanning to collect size metadata
- Add to `ImageStat`:
  ```go
  type ImageStat struct {
      Image    string `json:"image"`
      Count    int    `json:"count"`
      SizeBytes int64 `json:"size_bytes"` // NEW
  }
  ```
- Store in `image_stats` table
- Calculate averages over time in telemetry collector
- Create API endpoint `/api/stats/image-size-trends`
- Build line chart showing size trends

**Benefit:** Shows optimization efforts or bloat trends
**Note:** Image sizes can be large values, consider MB/GB formatting

---

### 11. Active Hours Heatmap
**Status:** ✅ **IMPLEMENTED**
See "Implemented Charts" section above.

---

### 12. Container Restart Frequency
**Status:** ⚠️ Requires state tracking
**Description:** Average restarts per container, indicates stability
**Current Limitation:** No restart tracking between scans
**Required Changes:**
- Track container restart count from Docker API
- Store historical data to detect changes between scans
- Add to `Container` model:
  ```go
  type Container struct {
      // ... existing fields
      RestartCount int `json:"restart_count"`
  }
  ```
- Aggregate restart stats in telemetry
- Add to `TelemetryReport`:
  ```go
  type TelemetryReport struct {
      // ... existing fields
      AvgRestarts float64 `json:"avg_restarts_per_container"`
      HighRestartContainers int `json:"high_restart_count"` // containers with >10 restarts
  }
  ```
- Create API endpoint `/api/stats/restart-frequency`
- Build bar chart or metric card

**Benefit:** Indicates stability issues and problematic containers

---

## Implementation Priority

### High Priority (Easy to implement, high value)
1. ✅ **Scan Interval Distribution** - Already implemented
2. ✅ **Activity Hours Heatmap** - Already implemented
3. **Container State Distribution** - Simple aggregation, useful insight

### Medium Priority (Moderate effort, good value)
4. **Image Size Trends** - Requires metadata collection
5. **Container Restart Frequency** - Useful stability metric
6. **Geographic Distribution (timezone-based)** - Privacy-friendly, interesting metric

### Lower Priority (High effort or privacy concerns)
7. **Resource Usage Trends** - Adds scan overhead, requires time-series storage
8. **Geographic Distribution (IP-based)** - Privacy concerns, may conflict with anonymous telemetry goals

---

## Database Schema Changes

If implementing all enhancements, consider adding these fields to `telemetry_reports`:

```sql
ALTER TABLE telemetry_reports ADD COLUMN containers_running INTEGER DEFAULT 0;
ALTER TABLE telemetry_reports ADD COLUMN containers_stopped INTEGER DEFAULT 0;
ALTER TABLE telemetry_reports ADD COLUMN containers_paused INTEGER DEFAULT 0;
ALTER TABLE telemetry_reports ADD COLUMN avg_cpu_percent REAL DEFAULT 0.0;
ALTER TABLE telemetry_reports ADD COLUMN avg_memory_bytes BIGINT DEFAULT 0;
ALTER TABLE telemetry_reports ADD COLUMN avg_restarts REAL DEFAULT 0.0;
ALTER TABLE telemetry_reports ADD COLUMN timezone VARCHAR(100);
ALTER TABLE telemetry_reports ADD COLUMN region VARCHAR(50);

-- For image sizes
ALTER TABLE image_stats ADD COLUMN size_bytes BIGINT DEFAULT 0;
```

---

## Testing Considerations

When implementing these features:

1. **Performance:** Test scan time impact with resource collection
2. **Privacy:** Ensure no PII is collected in geographic data
3. **Storage:** Monitor database growth with additional fields
4. **Backwards Compatibility:** Ensure older clients can still report without new fields
5. **Data Validation:** Validate all numeric ranges (e.g., CPU 0-100%, valid timezones)

---

## Current Implementation Notes

**Charts Implemented (2 new charts):**
- Activity Hours Heatmap (item #11 from suggestion list)
- Scan Interval Distribution (simplified version of item #2)

**API Endpoints Added:**
- `GET /api/stats/activity-heatmap?days=30` - Returns telemetry report activity by day/hour
- `GET /api/stats/scan-intervals` - Returns distribution of scan interval configurations

**Frontend Updates:**
- Added bubble chart heatmap for activity patterns (7 days x 24 hours grid)
- Added doughnut chart for scan interval distribution
- Both charts use the vibrant color palette and smooth animations
- Responsive design with chart-row grid layout
