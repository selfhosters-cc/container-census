package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// GetDefaultSettings returns default system settings
func GetDefaultSettings() *models.SystemSettings {
	return &models.SystemSettings{
		Scanner: models.ScannerSettings{
			IntervalSeconds: 300, // 5 minutes
			TimeoutSeconds:  30,
		},
		Telemetry: models.TelemetrySettings{
			IntervalHours: 168, // 1 week
		},
		Notification: models.NotificationSettings{
			RateLimitMax:           100,
			RateLimitBatchInterval: 600, // 10 minutes
			ThresholdDuration:      120, // 2 minutes
			CooldownPeriod:         300, // 5 minutes
		},
		UpdatedAt: time.Now(),
	}
}

// IsFirstRun checks if system settings exist in the database
func (db *DB) IsFirstRun() bool {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM system_settings").Scan(&count)
	if err != nil {
		log.Printf("Error checking first run: %v", err)
		return true
	}
	return count == 0
}

// LoadSystemSettings loads all system settings from the database
func (db *DB) LoadSystemSettings() (*models.SystemSettings, error) {
	// Check if settings exist
	if db.IsFirstRun() {
		// Return defaults if no settings in DB
		return GetDefaultSettings(), nil
	}

	settings := &models.SystemSettings{
		Scanner:      models.ScannerSettings{},
		Telemetry:    models.TelemetrySettings{},
		Notification: models.NotificationSettings{},
	}

	// Load scanner settings
	if err := db.loadCategorySetting("scanner", "interval_seconds", &settings.Scanner.IntervalSeconds); err != nil {
		settings.Scanner.IntervalSeconds = 300 // Default
	}
	if err := db.loadCategorySetting("scanner", "timeout_seconds", &settings.Scanner.TimeoutSeconds); err != nil {
		settings.Scanner.TimeoutSeconds = 30 // Default
	}

	// Load telemetry settings
	if err := db.loadCategorySetting("telemetry", "interval_hours", &settings.Telemetry.IntervalHours); err != nil {
		settings.Telemetry.IntervalHours = 168 // Default
	}

	// Load notification settings
	if err := db.loadCategorySetting("notification", "rate_limit_max", &settings.Notification.RateLimitMax); err != nil {
		settings.Notification.RateLimitMax = 100 // Default
	}
	if err := db.loadCategorySetting("notification", "rate_limit_batch_interval", &settings.Notification.RateLimitBatchInterval); err != nil {
		settings.Notification.RateLimitBatchInterval = 600 // Default
	}
	if err := db.loadCategorySetting("notification", "threshold_duration", &settings.Notification.ThresholdDuration); err != nil {
		settings.Notification.ThresholdDuration = 120 // Default
	}
	if err := db.loadCategorySetting("notification", "cooldown_period", &settings.Notification.CooldownPeriod); err != nil {
		settings.Notification.CooldownPeriod = 300 // Default
	}

	// Get most recent update time
	var updatedAt string
	err := db.conn.QueryRow(`
		SELECT MAX(updated_at) FROM system_settings
	`).Scan(&updatedAt)
	if err == nil && updatedAt != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", updatedAt); err == nil {
			settings.UpdatedAt = t
		}
	}

	return settings, nil
}

// loadCategorySetting loads a single setting value
func (db *DB) loadCategorySetting(category, key string, dest interface{}) error {
	var value string
	err := db.conn.QueryRow(`
		SELECT value FROM system_settings
		WHERE category = ? AND key = ?
	`, category, key).Scan(&value)

	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(value), dest)
}

// SaveSystemSettings saves all system settings to the database
func (db *DB) SaveSystemSettings(settings *models.SystemSettings) error {
	// Validate settings before saving
	if err := settings.Validate(); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	settings.UpdatedAt = now

	// Save scanner settings
	if err := db.saveSetting(tx, "scanner", "interval_seconds", settings.Scanner.IntervalSeconds, "int", "Scan interval in seconds", now); err != nil {
		return err
	}
	if err := db.saveSetting(tx, "scanner", "timeout_seconds", settings.Scanner.TimeoutSeconds, "int", "Scan timeout in seconds", now); err != nil {
		return err
	}

	// Save telemetry settings
	if err := db.saveSetting(tx, "telemetry", "interval_hours", settings.Telemetry.IntervalHours, "int", "Telemetry submission interval in hours", now); err != nil {
		return err
	}

	// Save notification settings
	if err := db.saveSetting(tx, "notification", "rate_limit_max", settings.Notification.RateLimitMax, "int", "Maximum notifications per hour", now); err != nil {
		return err
	}
	if err := db.saveSetting(tx, "notification", "rate_limit_batch_interval", settings.Notification.RateLimitBatchInterval, "int", "Batch interval in seconds when rate limited", now); err != nil {
		return err
	}
	if err := db.saveSetting(tx, "notification", "threshold_duration", settings.Notification.ThresholdDuration, "int", "Threshold breach duration in seconds before alerting", now); err != nil {
		return err
	}
	if err := db.saveSetting(tx, "notification", "cooldown_period", settings.Notification.CooldownPeriod, "int", "Cooldown between alerts for same container in seconds", now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("System settings saved successfully")
	return nil
}

// saveSetting saves a single setting to the database
func (db *DB) saveSetting(tx *sql.Tx, category, key string, value interface{}, dataType, description string, updatedAt time.Time) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value for %s.%s: %w", category, key, err)
	}

	_, err = tx.Exec(`
		INSERT INTO system_settings (category, key, value, data_type, description, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(category, key) DO UPDATE SET
			value = excluded.value,
			data_type = excluded.data_type,
			description = excluded.description,
			updated_at = excluded.updated_at
	`, category, key, string(valueJSON), dataType, description, updatedAt)

	if err != nil {
		return fmt.Errorf("failed to save setting %s.%s: %w", category, key, err)
	}

	return nil
}

// GetSetting retrieves a single setting value
func (db *DB) GetSetting(category, key string) (interface{}, error) {
	var value string
	err := db.conn.QueryRow(`
		SELECT value FROM system_settings
		WHERE category = ? AND key = ?
	`, category, key).Scan(&value)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("setting not found: %s.%s", category, key)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get setting %s.%s: %w", category, key, err)
	}

	var result interface{}
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal setting %s.%s: %w", category, key, err)
	}

	return result, nil
}

// SetSetting updates a single setting value
func (db *DB) SetSetting(category, key string, value interface{}) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	result, err := db.conn.Exec(`
		UPDATE system_settings
		SET value = ?, updated_at = ?
		WHERE category = ? AND key = ?
	`, string(valueJSON), time.Now(), category, key)

	if err != nil {
		return fmt.Errorf("failed to update setting: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("setting not found: %s.%s", category, key)
	}

	return nil
}

// GetSettingsByCategory retrieves all settings for a category
func (db *DB) GetSettingsByCategory(category string) (map[string]interface{}, error) {
	rows, err := db.conn.Query(`
		SELECT key, value FROM system_settings
		WHERE category = ?
		ORDER BY key
	`, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]interface{})
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}

		var val interface{}
		if err := json.Unmarshal([]byte(value), &val); err != nil {
			log.Printf("Warning: failed to unmarshal %s.%s: %v", category, key, err)
			continue
		}

		settings[key] = val
	}

	return settings, nil
}

// GetTelemetryEndpoints retrieves all telemetry endpoints from the database
func (db *DB) GetTelemetryEndpoints() ([]models.TelemetryEndpoint, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, url, enabled, api_key, last_success, last_error, created_at, updated_at
		FROM telemetry_endpoints
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query telemetry endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []models.TelemetryEndpoint
	for rows.Next() {
		var ep models.TelemetryEndpoint
		var id int64
		var lastSuccess, createdAt, updatedAt sql.NullString
		var apiKey, lastError sql.NullString

		err := rows.Scan(&id, &ep.Name, &ep.URL, &ep.Enabled, &apiKey, &lastSuccess, &lastError, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan endpoint: %w", err)
		}

		if apiKey.Valid {
			ep.APIKey = apiKey.String
		}
		if lastError.Valid {
			ep.LastFailureReason = lastError.String
		}
		if lastSuccess.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", lastSuccess.String); err == nil {
				ep.LastSuccess = &t
			}
		}

		endpoints = append(endpoints, ep)
	}

	return endpoints, nil
}

// SaveTelemetryEndpoint saves or updates a telemetry endpoint
func (db *DB) SaveTelemetryEndpoint(endpoint *models.TelemetryEndpoint) error {
	_, err := db.conn.Exec(`
		INSERT INTO telemetry_endpoints (name, url, enabled, api_key, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			url = excluded.url,
			enabled = excluded.enabled,
			api_key = excluded.api_key,
			updated_at = excluded.updated_at
	`, endpoint.Name, endpoint.URL, endpoint.Enabled, endpoint.APIKey, time.Now())

	if err != nil {
		return fmt.Errorf("failed to save telemetry endpoint: %w", err)
	}

	return nil
}

// DeleteTelemetryEndpoint deletes a telemetry endpoint by name
func (db *DB) DeleteTelemetryEndpoint(name string) error {
	result, err := db.conn.Exec(`
		DELETE FROM telemetry_endpoints WHERE name = ?
	`, name)
	if err != nil {
		return fmt.Errorf("failed to delete telemetry endpoint: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("telemetry endpoint not found: %s", name)
	}

	return nil
}
