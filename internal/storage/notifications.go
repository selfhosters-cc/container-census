package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// Notification operations

// GetNotificationChannels retrieves all notification channels
func (db *DB) GetNotificationChannels() ([]models.NotificationChannel, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, type, config, enabled, created_at, updated_at
		FROM notification_channels
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.NotificationChannel
	for rows.Next() {
		var ch models.NotificationChannel
		var configJSON string

		err := rows.Scan(&ch.ID, &ch.Name, &ch.Type, &configJSON, &ch.Enabled, &ch.CreatedAt, &ch.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(configJSON), &ch.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal channel config: %w", err)
		}

		channels = append(channels, ch)
	}

	return channels, rows.Err()
}

// GetNotificationChannel retrieves a single notification channel
func (db *DB) GetNotificationChannel(id int64) (*models.NotificationChannel, error) {
	var ch models.NotificationChannel
	var configJSON string

	err := db.conn.QueryRow(`
		SELECT id, name, type, config, enabled, created_at, updated_at
		FROM notification_channels
		WHERE id = ?
	`, id).Scan(&ch.ID, &ch.Name, &ch.Type, &configJSON, &ch.Enabled, &ch.CreatedAt, &ch.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(configJSON), &ch.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal channel config: %w", err)
	}

	return &ch, nil
}

// SaveNotificationChannel saves a notification channel
func (db *DB) SaveNotificationChannel(ch *models.NotificationChannel) error {
	configJSON, err := json.Marshal(ch.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if ch.ID == 0 {
		// Insert
		result, err := db.conn.Exec(`
			INSERT INTO notification_channels (name, type, config, enabled)
			VALUES (?, ?, ?, ?)
		`, ch.Name, ch.Type, string(configJSON), ch.Enabled)
		if err != nil {
			return err
		}
		ch.ID, _ = result.LastInsertId()
	} else {
		// Update
		_, err := db.conn.Exec(`
			UPDATE notification_channels
			SET name = ?, type = ?, config = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, ch.Name, ch.Type, string(configJSON), ch.Enabled, ch.ID)
		return err
	}

	return nil
}

// DeleteNotificationChannel deletes a notification channel
func (db *DB) DeleteNotificationChannel(id int64) error {
	_, err := db.conn.Exec("DELETE FROM notification_channels WHERE id = ?", id)
	return err
}

// GetNotificationRules retrieves notification rules
func (db *DB) GetNotificationRules(enabledOnly bool) ([]models.NotificationRule, error) {
	query := `
		SELECT r.id, r.name, r.enabled, r.event_types, r.host_id, r.container_pattern, r.image_pattern,
		       r.cpu_threshold, r.memory_threshold, r.threshold_duration_seconds, r.cooldown_seconds,
		       r.created_at, r.updated_at
		FROM notification_rules r
	`
	if enabledOnly {
		query += " WHERE r.enabled = 1"
	}
	query += " ORDER BY r.name"

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.NotificationRule
	for rows.Next() {
		var rule models.NotificationRule
		var eventTypesJSON string
		var hostID sql.NullInt64
		var containerPattern, imagePattern sql.NullString
		var cpuThreshold, memoryThreshold sql.NullFloat64

		err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Enabled, &eventTypesJSON, &hostID,
			&containerPattern, &imagePattern, &cpuThreshold, &memoryThreshold,
			&rule.ThresholdDurationSeconds, &rule.CooldownSeconds,
			&rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(eventTypesJSON), &rule.EventTypes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event types: %w", err)
		}

		if hostID.Valid {
			id := hostID.Int64
			rule.HostID = &id
		}
		if containerPattern.Valid {
			rule.ContainerPattern = containerPattern.String
		}
		if imagePattern.Valid {
			rule.ImagePattern = imagePattern.String
		}
		if cpuThreshold.Valid {
			threshold := cpuThreshold.Float64
			rule.CPUThreshold = &threshold
		}
		if memoryThreshold.Valid {
			threshold := memoryThreshold.Float64
			rule.MemoryThreshold = &threshold
		}

		// Get associated channels
		channelIDs, err := db.GetRuleChannels(rule.ID)
		if err != nil {
			return nil, err
		}
		rule.ChannelIDs = channelIDs

		rules = append(rules, rule)
	}

	return rules, rows.Err()
}

// GetRuleChannels retrieves channel IDs for a rule
func (db *DB) GetRuleChannels(ruleID int64) ([]int64, error) {
	rows, err := db.conn.Query(`
		SELECT channel_id FROM notification_rule_channels WHERE rule_id = ?
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channelIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		channelIDs = append(channelIDs, id)
	}

	return channelIDs, rows.Err()
}

// SaveNotificationRule saves a notification rule
func (db *DB) SaveNotificationRule(rule *models.NotificationRule) error {
	eventTypesJSON, err := json.Marshal(rule.EventTypes)
	if err != nil {
		return fmt.Errorf("failed to marshal event types: %w", err)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if rule.ID == 0 {
		// Insert
		result, err := tx.Exec(`
			INSERT INTO notification_rules
			(name, enabled, event_types, host_id, container_pattern, image_pattern,
			 cpu_threshold, memory_threshold, threshold_duration_seconds, cooldown_seconds)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, rule.Name, rule.Enabled, string(eventTypesJSON), rule.HostID,
			rule.ContainerPattern, rule.ImagePattern, rule.CPUThreshold, rule.MemoryThreshold,
			rule.ThresholdDurationSeconds, rule.CooldownSeconds)
		if err != nil {
			return err
		}
		rule.ID, _ = result.LastInsertId()
	} else {
		// Update
		_, err := tx.Exec(`
			UPDATE notification_rules
			SET name = ?, enabled = ?, event_types = ?, host_id = ?,
			    container_pattern = ?, image_pattern = ?, cpu_threshold = ?, memory_threshold = ?,
			    threshold_duration_seconds = ?, cooldown_seconds = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, rule.Name, rule.Enabled, string(eventTypesJSON), rule.HostID,
			rule.ContainerPattern, rule.ImagePattern, rule.CPUThreshold, rule.MemoryThreshold,
			rule.ThresholdDurationSeconds, rule.CooldownSeconds, rule.ID)
		if err != nil {
			return err
		}

		// Delete existing channel associations
		if _, err := tx.Exec("DELETE FROM notification_rule_channels WHERE rule_id = ?", rule.ID); err != nil {
			return err
		}
	}

	// Insert channel associations
	for _, channelID := range rule.ChannelIDs {
		_, err := tx.Exec(`
			INSERT INTO notification_rule_channels (rule_id, channel_id) VALUES (?, ?)
		`, rule.ID, channelID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteNotificationRule deletes a notification rule
func (db *DB) DeleteNotificationRule(id int64) error {
	_, err := db.conn.Exec("DELETE FROM notification_rules WHERE id = ?", id)
	return err
}

// SaveNotificationLog saves a notification log entry
func (db *DB) SaveNotificationLog(log models.NotificationLog) error {
	metadataJSON, err := json.Marshal(log.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	_, err = db.conn.Exec(`
		INSERT INTO notification_log
		(rule_id, channel_id, event_type, container_id, container_name, host_id, host_name,
		 message, metadata, sent_at, success, error, read)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.RuleID, log.ChannelID, log.EventType, log.ContainerID, log.ContainerName,
		log.HostID, log.HostName, log.Message, string(metadataJSON), log.SentAt,
		log.Success, log.Error, log.Read)

	return err
}

// GetNotificationLogs retrieves notification logs
func (db *DB) GetNotificationLogs(limit int, unreadOnly bool) ([]models.NotificationLog, error) {
	query := `
		SELECT l.id, l.rule_id, l.channel_id, l.event_type, l.container_id, l.container_name,
		       l.host_id, l.host_name, l.message, l.metadata, l.sent_at, l.success, l.error, l.read
		FROM notification_log l
	`
	if unreadOnly {
		query += " WHERE l.read = 0"
	}
	query += " ORDER BY l.sent_at DESC LIMIT ?"

	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.NotificationLog
	for rows.Next() {
		var log models.NotificationLog
		var ruleID, channelID, hostID sql.NullInt64
		var containerID, containerName, hostName, errorMsg, metadataJSON sql.NullString

		err := rows.Scan(
			&log.ID, &ruleID, &channelID, &log.EventType, &containerID, &containerName,
			&hostID, &hostName, &log.Message, &metadataJSON, &log.SentAt,
			&log.Success, &errorMsg, &log.Read,
		)
		if err != nil {
			return nil, err
		}

		if ruleID.Valid {
			id := ruleID.Int64
			log.RuleID = &id
		}
		if channelID.Valid {
			id := channelID.Int64
			log.ChannelID = &id
		}
		if hostID.Valid {
			id := hostID.Int64
			log.HostID = &id
		}
		if containerID.Valid {
			log.ContainerID = containerID.String
		}
		if containerName.Valid {
			log.ContainerName = containerName.String
		}
		if hostName.Valid {
			log.HostName = hostName.String
		}
		if errorMsg.Valid {
			log.Error = errorMsg.String
		}
		if metadataJSON.Valid && metadataJSON.String != "" && metadataJSON.String != "{}" {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
				log.Metadata = metadata
			}
		}

		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// MarkNotificationRead marks a notification as read
func (db *DB) MarkNotificationRead(id int64) error {
	_, err := db.conn.Exec("UPDATE notification_log SET read = 1 WHERE id = ?", id)
	return err
}

// MarkAllNotificationsRead marks all notifications as read
func (db *DB) MarkAllNotificationsRead() error {
	_, err := db.conn.Exec("UPDATE notification_log SET read = 1 WHERE read = 0")
	return err
}

// GetUnreadNotificationCount returns the count of unread notifications
func (db *DB) GetUnreadNotificationCount() (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM notification_log WHERE read = 0").Scan(&count)
	return count, err
}

// CleanupOldNotifications removes notifications older than 7 days or beyond the 100 most recent
func (db *DB) CleanupOldNotifications() error {
	// Keep last 100 notifications OR notifications from last 7 days, whichever is larger
	// This means: delete if (older than 7 days) AND (beyond the 100 most recent)

	// Get total count first
	var totalCount int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM notification_log").Scan(&totalCount)
	if err != nil {
		return err
	}

	// If we have 100 or fewer, only delete those older than 7 days
	if totalCount <= 100 {
		_, err := db.conn.Exec(`
			DELETE FROM notification_log
			WHERE sent_at < datetime('now', '-7 days')
		`)
		return err
	}

	// If we have more than 100, delete records that are BOTH old AND beyond top 100
	_, err = db.conn.Exec(`
		DELETE FROM notification_log
		WHERE sent_at < datetime('now', '-7 days')
		  AND id NOT IN (
			SELECT id FROM notification_log
			ORDER BY sent_at DESC
			LIMIT 100
		  )
	`)
	return err
}

// ClearAllNotifications deletes all notifications (user-initiated action)
func (db *DB) ClearAllNotifications() error {
	_, err := db.conn.Exec(`DELETE FROM notification_log`)
	return err
}

// GetActiveSilences retrieves active notification silences
func (db *DB) GetActiveSilences() ([]models.NotificationSilence, error) {
	rows, err := db.conn.Query(`
		SELECT id, host_id, container_id, container_name, host_pattern, container_pattern,
		       silenced_until, reason, created_at
		FROM notification_silences
		WHERE silenced_until > datetime('now')
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var silences []models.NotificationSilence
	for rows.Next() {
		var s models.NotificationSilence
		var hostID sql.NullInt64
		var containerID, containerName, hostPattern, containerPattern, reason sql.NullString

		err := rows.Scan(
			&s.ID, &hostID, &containerID, &containerName, &hostPattern, &containerPattern,
			&s.SilencedUntil, &reason, &s.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if hostID.Valid {
			id := hostID.Int64
			s.HostID = &id
		}
		if containerID.Valid {
			s.ContainerID = containerID.String
		}
		if containerName.Valid {
			s.ContainerName = containerName.String
		}
		if hostPattern.Valid {
			s.HostPattern = hostPattern.String
		}
		if containerPattern.Valid {
			s.ContainerPattern = containerPattern.String
		}
		if reason.Valid {
			s.Reason = reason.String
		}

		silences = append(silences, s)
	}

	return silences, rows.Err()
}

// SaveNotificationSilence saves a notification silence
func (db *DB) SaveNotificationSilence(silence *models.NotificationSilence) error {
	result, err := db.conn.Exec(`
		INSERT INTO notification_silences
		(host_id, container_id, container_name, host_pattern, container_pattern, silenced_until, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, silence.HostID, silence.ContainerID, silence.ContainerName, silence.HostPattern,
		silence.ContainerPattern, silence.SilencedUntil, silence.Reason)

	if err != nil {
		return err
	}

	silence.ID, _ = result.LastInsertId()
	return nil
}

// DeleteNotificationSilence deletes a notification silence
func (db *DB) DeleteNotificationSilence(id int64) error {
	_, err := db.conn.Exec("DELETE FROM notification_silences WHERE id = ?", id)
	return err
}

// GetLastNotificationTime retrieves the last notification time for a rule/container/host combo
func (db *DB) GetLastNotificationTime(ruleID int64, containerID string, hostID int64) (*time.Time, error) {
	var lastTime sql.NullTime
	err := db.conn.QueryRow(`
		SELECT MAX(sent_at) FROM notification_log
		WHERE rule_id = ? AND container_id = ? AND host_id = ? AND success = 1
	`, ruleID, containerID, hostID).Scan(&lastTime)

	if err != nil || !lastTime.Valid {
		return nil, err
	}

	return &lastTime.Time, nil
}

// GetContainerBaseline retrieves baseline stats for a container
func (db *DB) GetContainerBaseline(containerID string, hostID int64) (*models.ContainerBaselineStats, error) {
	var baseline models.ContainerBaselineStats
	var avgCPU, avgMemoryPercent sql.NullFloat64
	var avgMemoryUsage sql.NullInt64

	err := db.conn.QueryRow(`
		SELECT id, container_id, container_name, host_id, image_id,
		       avg_cpu_percent, avg_memory_percent, avg_memory_usage,
		       sample_count, window_start, window_end, created_at
		FROM container_baseline_stats
		WHERE container_id = ? AND host_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, containerID, hostID).Scan(
		&baseline.ID, &baseline.ContainerID, &baseline.ContainerName, &baseline.HostID,
		&baseline.ImageID, &avgCPU, &avgMemoryPercent, &avgMemoryUsage,
		&baseline.SampleCount, &baseline.WindowStart, &baseline.WindowEnd, &baseline.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if avgCPU.Valid {
		baseline.AvgCPUPercent = avgCPU.Float64
	}
	if avgMemoryPercent.Valid {
		baseline.AvgMemoryPercent = avgMemoryPercent.Float64
	}
	if avgMemoryUsage.Valid {
		baseline.AvgMemoryUsage = avgMemoryUsage.Int64
	}

	return &baseline, nil
}

// SaveContainerBaseline saves baseline stats for a container
func (db *DB) SaveContainerBaseline(baseline *models.ContainerBaselineStats) error {
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO container_baseline_stats
		(container_id, container_name, host_id, image_id, avg_cpu_percent, avg_memory_percent,
		 avg_memory_usage, sample_count, window_start, window_end)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, baseline.ContainerID, baseline.ContainerName, baseline.HostID, baseline.ImageID,
		baseline.AvgCPUPercent, baseline.AvgMemoryPercent, baseline.AvgMemoryUsage,
		baseline.SampleCount, baseline.WindowStart, baseline.WindowEnd)

	return err
}

// GetNotificationStatus returns the current notification system status
func (db *DB) GetNotificationStatus() (*models.NotificationStatus, error) {
	var status models.NotificationStatus

	// Unread count
	db.conn.QueryRow("SELECT COUNT(*) FROM notification_log WHERE read = 0").Scan(&status.UnreadCount)

	// Channel counts
	db.conn.QueryRow("SELECT COUNT(*) FROM notification_channels").Scan(&status.TotalChannels)
	db.conn.QueryRow("SELECT COUNT(*) FROM notification_channels WHERE enabled = 1").Scan(&status.EnabledChannels)

	// Rule counts
	db.conn.QueryRow("SELECT COUNT(*) FROM notification_rules").Scan(&status.TotalRules)
	db.conn.QueryRow("SELECT COUNT(*) FROM notification_rules WHERE enabled = 1").Scan(&status.EnabledRules)

	// Recent failures (last 24 hours)
	db.conn.QueryRow(`
		SELECT COUNT(*) FROM notification_log
		WHERE success = 0 AND sent_at > datetime('now', '-24 hours')
	`).Scan(&status.RecentFailures)

	// Active silences
	db.conn.QueryRow(`
		SELECT COUNT(*) FROM notification_silences
		WHERE silenced_until > datetime('now')
	`).Scan(&status.ActiveSilences)

	return &status, nil
}
