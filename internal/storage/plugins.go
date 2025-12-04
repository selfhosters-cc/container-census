package storage

import (
	"database/sql"
	"encoding/json"
	"time"
)

// PluginRecord represents a plugin in the database
type PluginRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	SourceType  string    `json:"source_type"`
	SourceURL   string    `json:"source_url,omitempty"`
	Enabled     bool      `json:"enabled"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TabConfig   string    `json:"tab_config,omitempty"` // JSON string of tab configuration
}

// ExternalPluginRecord represents an external plugin with additional metadata
type ExternalPluginRecord struct {
	PluginRecord
	BinaryPath      string            `json:"binary_path,omitempty"`
	GRPCPort        int               `json:"grpc_port,omitempty"`
	ProcessStatus   string            `json:"process_status,omitempty"` // running, stopped, failed
	Permissions     []string          `json:"permissions,omitempty"`
	FrontendBundle  string            `json:"frontend_bundle,omitempty"`
	FrontendCSS     string            `json:"frontend_css,omitempty"`
	TabConfig       map[string]string `json:"tab_config,omitempty"`
}

// initPluginSchema creates the plugin-related database tables
func (db *DB) initPluginSchema() error {
	schema := `
	-- Plugin registry
	CREATE TABLE IF NOT EXISTS plugins (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		source_type TEXT NOT NULL,
		source_url TEXT,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		-- External plugin fields
		binary_path TEXT,
		grpc_port INTEGER,
		process_status TEXT,
		permissions TEXT,
		frontend_bundle TEXT,
		frontend_css TEXT,
		tab_config TEXT
	);

	-- Plugin key-value data storage (scoped per plugin)
	CREATE TABLE IF NOT EXISTS plugin_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plugin_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value BLOB,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(plugin_id, key)
	);

	CREATE INDEX IF NOT EXISTS idx_plugin_data_plugin_id ON plugin_data(plugin_id);
	CREATE INDEX IF NOT EXISTS idx_plugin_data_key ON plugin_data(plugin_id, key);

	-- Plugin settings (separate from data for UI display)
	CREATE TABLE IF NOT EXISTS plugin_settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plugin_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(plugin_id, key)
	);

	CREATE INDEX IF NOT EXISTS idx_plugin_settings_plugin_id ON plugin_settings(plugin_id);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: Add columns if they don't exist (for existing databases)
	migrations := []string{
		`ALTER TABLE plugins ADD COLUMN binary_path TEXT`,
		`ALTER TABLE plugins ADD COLUMN grpc_port INTEGER`,
		`ALTER TABLE plugins ADD COLUMN process_status TEXT`,
		`ALTER TABLE plugins ADD COLUMN permissions TEXT`,
		`ALTER TABLE plugins ADD COLUMN frontend_bundle TEXT`,
		`ALTER TABLE plugins ADD COLUMN frontend_css TEXT`,
		`ALTER TABLE plugins ADD COLUMN tab_config TEXT`,
	}

	for _, migration := range migrations {
		// Ignore errors - column may already exist
		db.conn.Exec(migration)
	}

	return nil
}

// GetPlugin retrieves a plugin by ID
func (db *DB) GetPlugin(id string) (*PluginRecord, error) {
	query := `
		SELECT id, name, version, source_type, COALESCE(source_url, ''), enabled, installed_at, updated_at
		FROM plugins
		WHERE id = ?
	`

	var record PluginRecord
	err := db.conn.QueryRow(query, id).Scan(
		&record.ID,
		&record.Name,
		&record.Version,
		&record.SourceType,
		&record.SourceURL,
		&record.Enabled,
		&record.InstalledAt,
		&record.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// SavePlugin saves or updates a plugin record
func (db *DB) SavePlugin(record *PluginRecord) error {
	query := `
		INSERT INTO plugins (id, name, version, source_type, source_url, enabled, installed_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			version = excluded.version,
			source_type = excluded.source_type,
			source_url = excluded.source_url,
			updated_at = excluded.updated_at
	`

	_, err := db.conn.Exec(query,
		record.ID,
		record.Name,
		record.Version,
		record.SourceType,
		record.SourceURL,
		record.Enabled,
		record.InstalledAt,
		record.UpdatedAt,
	)

	return err
}

// GetAllPlugins retrieves all plugin records
func (db *DB) GetAllPlugins() ([]*PluginRecord, error) {
	query := `
		SELECT id, name, version, source_type, COALESCE(source_url, ''), enabled, installed_at, updated_at, COALESCE(tab_config, '')
		FROM plugins
		ORDER BY name
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*PluginRecord
	for rows.Next() {
		var record PluginRecord
		err := rows.Scan(
			&record.ID,
			&record.Name,
			&record.Version,
			&record.SourceType,
			&record.SourceURL,
			&record.Enabled,
			&record.InstalledAt,
			&record.UpdatedAt,
			&record.TabConfig,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, &record)
	}

	return records, rows.Err()
}

// SetPluginEnabled enables or disables a plugin
func (db *DB) SetPluginEnabled(id string, enabled bool) error {
	query := `UPDATE plugins SET enabled = ?, updated_at = ? WHERE id = ?`
	_, err := db.conn.Exec(query, enabled, time.Now(), id)
	return err
}

// GetPluginData retrieves a plugin data value by key
func (db *DB) GetPluginData(pluginID, key string) ([]byte, error) {
	query := `SELECT value FROM plugin_data WHERE plugin_id = ? AND key = ?`

	var value []byte
	err := db.conn.QueryRow(query, pluginID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return value, err
}

// SetPluginData stores a plugin data value
func (db *DB) SetPluginData(pluginID, key string, value []byte) error {
	query := `
		INSERT INTO plugin_data (plugin_id, key, value, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(plugin_id, key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`

	_, err := db.conn.Exec(query, pluginID, key, value, time.Now())
	return err
}

// DeletePluginData removes a plugin data value
func (db *DB) DeletePluginData(pluginID, key string) error {
	query := `DELETE FROM plugin_data WHERE plugin_id = ? AND key = ?`
	_, err := db.conn.Exec(query, pluginID, key)
	return err
}

// ListPluginData returns all data with a key prefix for a plugin
func (db *DB) ListPluginData(pluginID, prefix string) (map[string][]byte, error) {
	query := `SELECT key, value FROM plugin_data WHERE plugin_id = ? AND key LIKE ?`

	rows, err := db.conn.Query(query, pluginID, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]byte)
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}

	return result, rows.Err()
}

// GetPluginSetting retrieves a plugin setting value
func (db *DB) GetPluginSetting(pluginID, key string) (string, error) {
	query := `SELECT value FROM plugin_settings WHERE plugin_id = ? AND key = ?`

	var value string
	err := db.conn.QueryRow(query, pluginID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetPluginSetting stores a plugin setting value
func (db *DB) SetPluginSetting(pluginID, key, value string) error {
	query := `
		INSERT INTO plugin_settings (plugin_id, key, value, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(plugin_id, key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`

	_, err := db.conn.Exec(query, pluginID, key, value, time.Now())
	return err
}

// GetAllPluginSettings retrieves all settings for a plugin
func (db *DB) GetAllPluginSettings(pluginID string) (map[string]string, error) {
	query := `SELECT key, value FROM plugin_settings WHERE plugin_id = ?`

	rows, err := db.conn.Query(query, pluginID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}

	return result, rows.Err()
}

// DeleteAllPluginData removes all data for a plugin
func (db *DB) DeleteAllPluginData(pluginID string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete settings
	if _, err := tx.Exec(`DELETE FROM plugin_settings WHERE plugin_id = ?`, pluginID); err != nil {
		return err
	}

	// Delete data
	if _, err := tx.Exec(`DELETE FROM plugin_data WHERE plugin_id = ?`, pluginID); err != nil {
		return err
	}

	return tx.Commit()
}

// DeletePlugin removes a plugin and all its data
func (db *DB) DeletePlugin(pluginID string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete settings
	if _, err := tx.Exec(`DELETE FROM plugin_settings WHERE plugin_id = ?`, pluginID); err != nil {
		return err
	}

	// Delete data
	if _, err := tx.Exec(`DELETE FROM plugin_data WHERE plugin_id = ?`, pluginID); err != nil {
		return err
	}

	// Delete plugin record
	if _, err := tx.Exec(`DELETE FROM plugins WHERE id = ?`, pluginID); err != nil {
		return err
	}

	return tx.Commit()
}

// GetExternalPlugin retrieves an external plugin with additional metadata
func (db *DB) GetExternalPlugin(id string) (*ExternalPluginRecord, error) {
	query := `
		SELECT id, name, version, source_type, COALESCE(source_url, ''), enabled,
		       installed_at, updated_at,
		       COALESCE(binary_path, ''), COALESCE(grpc_port, 0), COALESCE(process_status, ''),
		       COALESCE(permissions, ''), COALESCE(frontend_bundle, ''),
		       COALESCE(frontend_css, ''), COALESCE(tab_config, '')
		FROM plugins
		WHERE id = ?
	`

	var record ExternalPluginRecord
	var permissionsJSON, tabConfigJSON string

	err := db.conn.QueryRow(query, id).Scan(
		&record.ID,
		&record.Name,
		&record.Version,
		&record.SourceType,
		&record.SourceURL,
		&record.Enabled,
		&record.InstalledAt,
		&record.UpdatedAt,
		&record.BinaryPath,
		&record.GRPCPort,
		&record.ProcessStatus,
		&permissionsJSON,
		&record.FrontendBundle,
		&record.FrontendCSS,
		&tabConfigJSON,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse JSON fields
	if permissionsJSON != "" {
		if err := json.Unmarshal([]byte(permissionsJSON), &record.Permissions); err != nil {
			// If unmarshal fails, log but continue with empty permissions
			record.Permissions = []string{}
		}
	} else {
		record.Permissions = []string{}
	}

	if tabConfigJSON != "" {
		if err := json.Unmarshal([]byte(tabConfigJSON), &record.TabConfig); err != nil {
			// If unmarshal fails, log but continue with empty config
			record.TabConfig = make(map[string]string)
		}
	} else {
		record.TabConfig = make(map[string]string)
	}

	return &record, nil
}

// SaveExternalPlugin saves or updates an external plugin record
func (db *DB) SaveExternalPlugin(record *ExternalPluginRecord) error {
	query := `
		INSERT INTO plugins (
			id, name, version, source_type, source_url, enabled, installed_at, updated_at,
			binary_path, grpc_port, process_status, permissions, frontend_bundle, frontend_css, tab_config
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			version = excluded.version,
			source_type = excluded.source_type,
			source_url = excluded.source_url,
			binary_path = excluded.binary_path,
			grpc_port = excluded.grpc_port,
			process_status = excluded.process_status,
			permissions = excluded.permissions,
			frontend_bundle = excluded.frontend_bundle,
			frontend_css = excluded.frontend_css,
			tab_config = excluded.tab_config,
			updated_at = excluded.updated_at
	`

	// Convert arrays/maps to JSON
	permissionsJSON := ""
	if len(record.Permissions) > 0 {
		if permData, err := json.Marshal(record.Permissions); err == nil {
			permissionsJSON = string(permData)
		}
	}

	tabConfigJSON := ""
	if len(record.TabConfig) > 0 {
		if tabData, err := json.Marshal(record.TabConfig); err == nil {
			tabConfigJSON = string(tabData)
		}
	}

	_, err := db.conn.Exec(query,
		record.ID,
		record.Name,
		record.Version,
		record.SourceType,
		record.SourceURL,
		record.Enabled,
		record.InstalledAt,
		record.UpdatedAt,
		record.BinaryPath,
		record.GRPCPort,
		record.ProcessStatus,
		permissionsJSON,
		record.FrontendBundle,
		record.FrontendCSS,
		tabConfigJSON,
	)

	return err
}
