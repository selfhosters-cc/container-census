# Notification System Implementation Status

## Overview
Comprehensive notification system for Container Census with webhooks, ntfy, and in-app notifications.

## Completed (Phases 1-2.3)

### ✅ Phase 1: Database Schema & Models

**Files Created/Modified:**
- `internal/storage/db.go` - Added 8 notification tables to schema:
  - `notification_channels` - Webhook/ntfy/in-app channel configurations
  - `notification_rules` - Event matching and threshold rules
  - `notification_rule_channels` - Many-to-many rule→channel mapping
  - `notification_log` - Sent notification history with read status
  - `notification_silences` - Muted hosts/containers with expiry
  - `container_baseline_stats` - Pre-update baselines for anomaly detection
  - `notification_threshold_state` - Threshold breach duration tracking

- `internal/models/models.go` - Added comprehensive notification models:
  - Event type constants (new_image, state_change, high_cpu, high_memory, anomalous_behavior)
  - Channel type constants (webhook, ntfy, in_app)
  - NotificationChannel with WebhookConfig/NtfyConfig
  - NotificationRule with pattern matching and thresholds
  - NotificationLog for history
  - NotificationSilence for muting
  - ContainerBaselineStats for anomaly detection
  - NotificationEvent for internal event passing
  - NotificationStatus for dashboard stats

### ✅ Phase 2: Notification Service Core

**Files Created:**

1. **`internal/notifications/notifier.go`** (600+ lines)
   - NotificationService - Main coordinator
   - ProcessEvents() - Entry point called after each scan
   - detectLifecycleEvents() - State changes & image updates
   - detectThresholdEvents() - CPU/memory threshold checking with duration requirement
   - detectAnomalies() - Post-update resource usage comparison
   - matchRules() - Pattern matching & filtering
   - filterSilenced() - Silence checking
   - sendNotifications() - Rate-limited delivery with batching
   - Threshold state tracking for duration requirements
   - Cooldown management per rule/container/host

2. **`internal/notifications/ratelimiter.go`**
   - Token bucket rate limiting (default 100/hour)
   - Batch queue for rate-limited notifications
   - 10-minute batch interval (configurable)
   - Summary notifications when rate limited
   - Thread-safe with mutex protection

3. **`internal/notifications/channels/channel.go`**
   - Channel interface with Send(), Test(), Type(), Name()

4. **`internal/notifications/channels/webhook.go`**
   - HTTP POST to configured URL
   - Custom headers support
   - 3-attempt retry with exponential backoff
   - 10-second timeout
   - JSON payload with full event data

5. **`internal/notifications/channels/ntfy.go`**
   - Custom server URL support (default: ntfy.sh)
   - Bearer token authentication
   - Priority mapping by event type (1-5)
   - Emoji tags per event type
   - Topic-based routing
   - 3-attempt retry logic

6. **`internal/notifications/channels/inapp.go`**
   - Writes to notification_log table
   - No-op Send() (logging handled by notifier)
   - Test() creates sample notification

7. **`internal/storage/notifications.go`** (550+ lines)
   - GetNotificationChannels() / GetNotificationChannel()
   - SaveNotificationChannel() - Insert/update with JSON config
   - DeleteNotificationChannel()
   - GetNotificationRules() - With channel ID population
   - SaveNotificationRule() - Transactional with channel associations
   - DeleteNotificationRule()
   - SaveNotificationLog() - With metadata JSON
   - GetNotificationLogs() - Filterable by read status
   - MarkNotificationRead() / MarkAllNotificationsRead()
   - GetUnreadNotificationCount()
   - CleanupOldNotifications() - 7 days OR 100 most recent
   - GetActiveSilences() / SaveNotificationSilence() / DeleteNotificationSilence()
   - GetLastNotificationTime() - For cooldown checks
   - GetContainerBaseline() / SaveContainerBaseline() - For anomaly detection
   - GetNotificationStatus() - Dashboard statistics

## Remaining Work

### ⏳ Phase 2.4: Baseline Stats Collector (2-3 hours)

**Need to Create:**
- `internal/notifications/baseline.go`:
  - UpdateBaselines() - Runs hourly
  - Queries last 48 hours of container stats
  - Calculates avg CPU%, avg memory%
  - Stores per (container_id, host_id, image_id)
  - Triggered on image_updated events
  - Background goroutine with ticker

### ⏳ Phase 3: Scanner Integration (1-2 hours)

**Need to Modify:**
- `cmd/server/main.go`:
  - Import notification service
  - Initialize NotificationService in main()
  - Call notificationService.ProcessEvents(hostID) after db.SaveContainers() in performScan()
  - Add runHourlyBaselineUpdate() background job
  - Pass config values (rate limit, thresholds) from environment

- Environment variables to add:
  - NOTIFICATION_THRESHOLD_DURATION (default 120)
  - NOTIFICATION_COOLDOWN_PERIOD (default 300)
  - NOTIFICATION_RATE_LIMIT_MAX (default 100)
  - NOTIFICATION_RATE_LIMIT_BATCH_INTERVAL (default 600)

### ⏳ Phase 4: REST API Endpoints (3-4 hours)

**Need to Modify:**
- `internal/api/handlers.go`:

**Channel Management:**
- GET /api/notifications/channels
- POST /api/notifications/channels (validate config, test connectivity)
- PUT /api/notifications/channels/{id}
- DELETE /api/notifications/channels/{id}
- POST /api/notifications/channels/{id}/test

**Rule Management:**
- GET /api/notifications/rules
- POST /api/notifications/rules
- PUT /api/notifications/rules/{id}
- DELETE /api/notifications/rules/{id}
- POST /api/notifications/rules/{id}/dry-run (simulate matches)

**Notification History:**
- GET /api/notifications/log?limit=100&unread=true
- PUT /api/notifications/log/{id}/read
- POST /api/notifications/log/read-all
- DELETE /api/notifications/log/clear

**Silences:**
- GET /api/notifications/silences
- POST /api/notifications/silences (host_id, container_id, duration)
- DELETE /api/notifications/silences/{id}

**Status:**
- GET /api/notifications/status

### ⏳ Phase 5: Frontend UI (4-5 hours)

**Need to Modify:**
- `web/index.html`:
  - Add bell icon to header with unread badge
  - Add notification dropdown (last 10)
  - Add Notifications tab to main navigation

- `web/app.js`:
  - Auto-refresh unread count every 30s
  - Notification badge component
  - Notification dropdown with mark-as-read
  - Full notifications page with table
  - Channel management UI (add/edit/delete/test modals)
  - Rule management UI (complex form with pattern matching)
  - Silence management UI
  - Container action: "Silence notifications" button

- `web/styles.css`:
  - Notification badge styles
  - Dropdown menu styles
  - Modal forms for channels/rules

### ⏳ Phase 6: Configuration & Documentation (1-2 hours)

**Need to Update:**
- `CLAUDE.md`:
  - Add Notification System Architecture section
  - Document event flow
  - Explain baseline stats and anomaly detection
  - API endpoint reference
  - Configuration examples

- Default rules on first startup:
  - "Container Stopped" (all hosts, webhook only, high priority)
  - "New Image Detected" (all hosts, in-app only, info)
  - "High Resource Usage" (CPU>80%, Memory>90%, 120s duration, in-app + webhook)

### ⏳ Phase 7: Testing & Polish (2-3 hours)

**Testing Checklist:**
- [ ] Create webhook.site channel and verify payload
- [ ] Set up ntfy.sh channel with custom server
- [ ] Trigger all event types manually
- [ ] Verify rate limiting works (set low limit)
- [ ] Test batching with queue overflow
- [ ] Verify silence functionality
- [ ] Test anomaly detection with controlled image update
- [ ] Verify threshold duration requirement (120s)
- [ ] Test cooldown periods
- [ ] Verify 7-day/100-notification retention
- [ ] Check auto-refresh of unread count
- [ ] Test mark-as-read functionality
- [ ] Verify pattern matching (glob patterns)

**Polish:**
- Error handling for channel send failures
- Retry logic verification
- Circuit breaker for failing channels
- Performance optimization for large notification logs
- Index tuning for queries

## Architecture Decisions

**Event Detection:** Polling-based (scans every N seconds), not real-time push
**Rate Limiting:** Token bucket with batching (prevents notification storms)
**Threshold Duration:** Requires sustained breach for 120s before alerting
**Cooldown:** Per-rule/container/host to prevent spam
**Anomaly Detection:** Statistical baseline (48hr window), 25% increase threshold
**Retention:** 7 days OR 100 most recent (whichever is larger)
**Silences:** Time-based with glob pattern support
**In-App:** Just another channel type writing to notification_log

## Key Features Implemented

✅ Multi-channel delivery (webhook, ntfy, in-app)
✅ Flexible rule engine with pattern matching
✅ CPU/memory threshold monitoring with duration
✅ Anomaly detection (post-update behavior changes)
✅ Lifecycle event detection (state changes, image updates)
✅ Rate limiting with batching
✅ Cooldown periods
✅ Silence management
✅ Read/unread tracking
✅ 7-day retention + 100-notification limit
✅ Retry logic (3 attempts with backoff)
✅ Test notifications
✅ Custom ntfy servers
✅ Custom webhook headers

## Next Steps

1. Implement baseline stats collector (2h)
2. Integrate with scanner (1h)
3. Add API endpoints (3h)
4. Build frontend UI (4h)
5. Test end-to-end (2h)
6. Update documentation (1h)

**Total Remaining:** ~13 hours

## Estimated Total Implementation Time
- Completed: 10-12 hours
- Remaining: 13 hours
- **Total: 23-25 hours**
