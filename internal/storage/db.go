package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/container-census/container-census/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

// DB handles database operations
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and initializes schema
func New(dbPath string) (*DB, error) {
	// Add SQLite parameters for better concurrency and time parsing
	// _parseTime=true: Parse TIME columns into time.Time
	// _busy_timeout=5000: Wait up to 5 seconds for locks
	// _journal_mode=WAL: Enable Write-Ahead Logging for better concurrency
	dsn := dbPath + "?_parseTime=true&_busy_timeout=5000&_journal_mode=WAL"
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool limits to prevent lock contention
	// Max 10 open connections, WAL mode allows multiple readers + 1 writer
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{conn: conn}

	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// initSchema creates the database tables
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS hosts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		address TEXT NOT NULL,
		description TEXT,
		host_type TEXT DEFAULT 'unknown',
		agent_token TEXT,
		agent_status TEXT DEFAULT 'unknown',
		last_seen TIMESTAMP,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		collect_stats BOOLEAN NOT NULL DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS containers (
		id TEXT NOT NULL,
		name TEXT NOT NULL,
		image TEXT NOT NULL,
		image_id TEXT NOT NULL,
		state TEXT NOT NULL,
		status TEXT NOT NULL,
		ports TEXT,
		labels TEXT,
		created TIMESTAMP NOT NULL,
		host_id INTEGER NOT NULL,
		host_name TEXT NOT NULL,
		scanned_at TIMESTAMP NOT NULL,
		networks TEXT,
		volumes TEXT,
		links TEXT,
		compose_project TEXT,
		cpu_percent REAL,
		memory_usage INTEGER,
		memory_limit INTEGER,
		memory_percent REAL,
		PRIMARY KEY (id, host_id, scanned_at),
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_containers_host_id ON containers(host_id);
	CREATE INDEX IF NOT EXISTS idx_containers_scanned_at ON containers(scanned_at);
	CREATE INDEX IF NOT EXISTS idx_containers_state ON containers(state);
	CREATE INDEX IF NOT EXISTS idx_containers_stats ON containers(id, host_id, scanned_at, state);

	CREATE TABLE IF NOT EXISTS container_stats_aggregates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		container_id TEXT NOT NULL,
		container_name TEXT NOT NULL,
		host_id INTEGER NOT NULL,
		host_name TEXT NOT NULL,
		timestamp_hour TIMESTAMP NOT NULL,
		avg_cpu_percent REAL,
		avg_memory_usage INTEGER,
		max_cpu_percent REAL,
		max_memory_usage INTEGER,
		sample_count INTEGER NOT NULL,
		UNIQUE(container_id, host_id, timestamp_hour),
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_stats_aggregates ON container_stats_aggregates(container_id, host_id, timestamp_hour);

	CREATE TABLE IF NOT EXISTS scan_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id INTEGER NOT NULL,
		host_name TEXT NOT NULL,
		started_at TIMESTAMP NOT NULL,
		completed_at TIMESTAMP NOT NULL,
		success BOOLEAN NOT NULL,
		error TEXT,
		containers_found INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_scan_results_host_id ON scan_results(host_id);
	CREATE INDEX IF NOT EXISTS idx_scan_results_started_at ON scan_results(started_at);

	CREATE TABLE IF NOT EXISTS telemetry_status (
		endpoint_name TEXT PRIMARY KEY,
		endpoint_url TEXT NOT NULL,
		last_success TIMESTAMP,
		last_failure TIMESTAMP,
		last_failure_reason TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS telemetry_submissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		endpoint_name TEXT NOT NULL,
		endpoint_url TEXT NOT NULL,
		started_at TIMESTAMP NOT NULL,
		completed_at TIMESTAMP NOT NULL,
		success BOOLEAN NOT NULL,
		error TEXT,
		hosts_count INTEGER,
		containers_count INTEGER,
		images_count INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_telemetry_submissions_created ON telemetry_submissions(created_at DESC);

	CREATE TABLE IF NOT EXISTS notification_channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL,
		config TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS notification_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		event_types TEXT NOT NULL,
		host_id INTEGER,
		container_pattern TEXT,
		image_pattern TEXT,
		cpu_threshold REAL,
		memory_threshold REAL,
		threshold_duration_seconds INTEGER DEFAULT 120,
		cooldown_seconds INTEGER DEFAULT 300,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS notification_rule_channels (
		rule_id INTEGER NOT NULL,
		channel_id INTEGER NOT NULL,
		PRIMARY KEY (rule_id, channel_id),
		FOREIGN KEY (rule_id) REFERENCES notification_rules(id) ON DELETE CASCADE,
		FOREIGN KEY (channel_id) REFERENCES notification_channels(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS notification_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rule_id INTEGER,
		channel_id INTEGER,
		event_type TEXT NOT NULL,
		container_id TEXT,
		container_name TEXT,
		host_id INTEGER,
		host_name TEXT,
		message TEXT NOT NULL,
		metadata TEXT,
		sent_at TIMESTAMP NOT NULL,
		success BOOLEAN NOT NULL,
		error TEXT,
		read BOOLEAN NOT NULL DEFAULT 0,
		FOREIGN KEY (rule_id) REFERENCES notification_rules(id) ON DELETE SET NULL,
		FOREIGN KEY (channel_id) REFERENCES notification_channels(id) ON DELETE SET NULL
	);

	CREATE INDEX IF NOT EXISTS idx_notification_log_sent_at ON notification_log(sent_at DESC);
	CREATE INDEX IF NOT EXISTS idx_notification_log_read ON notification_log(read, sent_at DESC);
	CREATE INDEX IF NOT EXISTS idx_notification_log_container ON notification_log(container_id, host_id);

	CREATE TABLE IF NOT EXISTS notification_silences (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id INTEGER,
		container_id TEXT,
		container_name TEXT,
		host_pattern TEXT,
		container_pattern TEXT,
		silenced_until TIMESTAMP NOT NULL,
		reason TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_notification_silences_until ON notification_silences(silenced_until);
	CREATE INDEX IF NOT EXISTS idx_notification_silences_container ON notification_silences(container_id, host_id);

	CREATE TABLE IF NOT EXISTS container_baseline_stats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		container_id TEXT NOT NULL,
		container_name TEXT NOT NULL,
		host_id INTEGER NOT NULL,
		image_id TEXT NOT NULL,
		avg_cpu_percent REAL,
		avg_memory_percent REAL,
		avg_memory_usage INTEGER,
		sample_count INTEGER NOT NULL,
		window_start TIMESTAMP NOT NULL,
		window_end TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(container_id, host_id, image_id),
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_baseline_stats_container ON container_baseline_stats(container_id, host_id, image_id);

	CREATE TABLE IF NOT EXISTS notification_threshold_state (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		container_id TEXT NOT NULL,
		host_id INTEGER NOT NULL,
		threshold_type TEXT NOT NULL,
		breached_at TIMESTAMP NOT NULL,
		last_notified_at TIMESTAMP,
		breach_count INTEGER NOT NULL DEFAULT 1,
		UNIQUE(container_id, host_id, threshold_type),
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_threshold_state_container ON notification_threshold_state(container_id, host_id);

	CREATE TABLE IF NOT EXISTS vulnerability_scans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		image_id TEXT NOT NULL UNIQUE,
		image_name TEXT NOT NULL,
		scanned_at TIMESTAMP NOT NULL,
		scan_duration_ms INTEGER NOT NULL,
		success BOOLEAN NOT NULL,
		error TEXT,
		trivy_db_version TEXT,
		total_vulnerabilities INTEGER DEFAULT 0,
		critical_count INTEGER DEFAULT 0,
		high_count INTEGER DEFAULT 0,
		medium_count INTEGER DEFAULT 0,
		low_count INTEGER DEFAULT 0,
		unknown_count INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_vuln_scans_image ON vulnerability_scans(image_id);
	CREATE INDEX IF NOT EXISTS idx_vuln_scans_scanned_at ON vulnerability_scans(scanned_at);

	CREATE TABLE IF NOT EXISTS vulnerabilities (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		image_id TEXT NOT NULL,
		vulnerability_id TEXT NOT NULL,
		pkg_name TEXT NOT NULL,
		installed_version TEXT,
		fixed_version TEXT,
		severity TEXT NOT NULL,
		title TEXT,
		description TEXT,
		published_date TIMESTAMP,
		last_modified_date TIMESTAMP,
		primary_url TEXT,
		FOREIGN KEY (image_id) REFERENCES vulnerability_scans(image_id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_vulns_image ON vulnerabilities(image_id);
	CREATE INDEX IF NOT EXISTS idx_vulns_severity ON vulnerabilities(severity);
	CREATE INDEX IF NOT EXISTS idx_vulns_cve ON vulnerabilities(vulnerability_id);

	CREATE TABLE IF NOT EXISTS image_containers (
		image_id TEXT NOT NULL,
		container_id TEXT NOT NULL,
		host_id INTEGER NOT NULL,
		last_seen TIMESTAMP NOT NULL,
		PRIMARY KEY (image_id, container_id, host_id)
	);

	CREATE INDEX IF NOT EXISTS idx_image_containers ON image_containers(image_id);

	CREATE TABLE IF NOT EXISTS vulnerability_settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		value TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS user_preferences (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return err
	}

	// Run migrations for existing databases
	return db.runMigrations()
}

// runMigrations handles schema updates for existing databases
func (db *DB) runMigrations() error {
	// Check if host_type column exists
	var columnExists int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('hosts') WHERE name='host_type'
	`).Scan(&columnExists)
	if err != nil {
		return err
	}

	// Add new columns if they don't exist (SQLite doesn't support IF NOT EXISTS for ALTER TABLE)
	if columnExists == 0 {
		migrations := []string{
			`ALTER TABLE hosts ADD COLUMN host_type TEXT DEFAULT 'unknown'`,
			`ALTER TABLE hosts ADD COLUMN agent_token TEXT`,
			`ALTER TABLE hosts ADD COLUMN agent_status TEXT DEFAULT 'unknown'`,
			`ALTER TABLE hosts ADD COLUMN last_seen TIMESTAMP`,
		}

		for _, migration := range migrations {
			if _, err := db.conn.Exec(migration); err != nil {
				// Ignore "duplicate column" errors
				if !isSQLiteColumnExistsError(err) {
					return err
				}
			}
		}
	}

	// Check if containers.networks column exists (for graph feature)
	var networksExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('containers') WHERE name='networks'
	`).Scan(&networksExists)
	if err != nil {
		return err
	}

	if networksExists == 0 {
		containerMigrations := []string{
			`ALTER TABLE containers ADD COLUMN networks TEXT`,
			`ALTER TABLE containers ADD COLUMN volumes TEXT`,
			`ALTER TABLE containers ADD COLUMN links TEXT`,
			`ALTER TABLE containers ADD COLUMN compose_project TEXT`,
		}

		for _, migration := range containerMigrations {
			if _, err := db.conn.Exec(migration); err != nil {
				// Ignore "duplicate column" errors
				if !isSQLiteContainerColumnExistsError(err) {
					return err
				}
			}
		}
	}

	// Check if collect_stats column exists in hosts table
	var collectStatsExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('hosts') WHERE name='collect_stats'
	`).Scan(&collectStatsExists)
	if err != nil {
		return err
	}

	if collectStatsExists == 0 {
		if _, err := db.conn.Exec(`ALTER TABLE hosts ADD COLUMN collect_stats BOOLEAN NOT NULL DEFAULT 1`); err != nil {
			if !isSQLiteColumnExistsError(err) {
				return err
			}
		}
	}

	// Check if cpu_percent column exists in containers table (for stats monitoring)
	var cpuPercentExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('containers') WHERE name='cpu_percent'
	`).Scan(&cpuPercentExists)
	if err != nil {
		return err
	}

	if cpuPercentExists == 0 {
		statsMigrations := []string{
			`ALTER TABLE containers ADD COLUMN cpu_percent REAL`,
			`ALTER TABLE containers ADD COLUMN memory_usage INTEGER`,
			`ALTER TABLE containers ADD COLUMN memory_limit INTEGER`,
			`ALTER TABLE containers ADD COLUMN memory_percent REAL`,
		}

		for _, migration := range statsMigrations {
			if _, err := db.conn.Exec(migration); err != nil {
				// Ignore "duplicate column" errors
				if !isSQLiteStatsColumnExistsError(err) {
					return err
				}
			}
		}
	}

	return nil
}

// isSQLiteColumnExistsError checks if error is about duplicate column
func isSQLiteColumnExistsError(err error) bool {
	return err != nil && (
		err.Error() == "duplicate column name: host_type" ||
		err.Error() == "duplicate column name: agent_token" ||
		err.Error() == "duplicate column name: agent_status" ||
		err.Error() == "duplicate column name: last_seen")
}

// isSQLiteContainerColumnExistsError checks if error is about duplicate container column
func isSQLiteContainerColumnExistsError(err error) bool {
	return err != nil && (
		err.Error() == "duplicate column name: networks" ||
		err.Error() == "duplicate column name: volumes" ||
		err.Error() == "duplicate column name: links" ||
		err.Error() == "duplicate column name: compose_project")
}

// isSQLiteStatsColumnExistsError checks if error is about duplicate stats column
func isSQLiteStatsColumnExistsError(err error) bool {
	return err != nil && (
		err.Error() == "duplicate column name: cpu_percent" ||
		err.Error() == "duplicate column name: memory_usage" ||
		err.Error() == "duplicate column name: memory_limit" ||
		err.Error() == "duplicate column name: memory_percent" ||
		err.Error() == "duplicate column name: collect_stats")
}

// Host operations

// AddHost adds a new host
func (db *DB) AddHost(host models.Host) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO hosts (name, address, description, host_type, agent_token, agent_status, last_seen, enabled, collect_stats)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		host.Name, host.Address, host.Description, host.HostType, host.AgentToken, host.AgentStatus, host.LastSeen, host.Enabled, host.CollectStats,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetHosts returns all hosts
func (db *DB) GetHosts() ([]models.Host, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, address, description, host_type, agent_token, agent_status, last_seen, enabled, collect_stats, created_at, updated_at
		FROM hosts
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []models.Host
	for rows.Next() {
		var h models.Host
		var lastSeen sql.NullTime
		var agentToken, agentStatus sql.NullString
		var collectStats sql.NullBool

		if err := rows.Scan(&h.ID, &h.Name, &h.Address, &h.Description, &h.HostType, &agentToken, &agentStatus, &lastSeen, &h.Enabled, &collectStats, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}

		if agentToken.Valid {
			h.AgentToken = agentToken.String
		}
		if agentStatus.Valid {
			h.AgentStatus = agentStatus.String
		}
		if lastSeen.Valid {
			h.LastSeen = lastSeen.Time
		}
		if collectStats.Valid {
			h.CollectStats = collectStats.Bool
		} else {
			h.CollectStats = true // Default to true
		}

		hosts = append(hosts, h)
	}

	return hosts, rows.Err()
}

// GetHost returns a single host by ID
func (db *DB) GetHost(id int64) (*models.Host, error) {
	var h models.Host
	var lastSeen sql.NullTime
	var agentToken, agentStatus sql.NullString
	var collectStats sql.NullBool

	err := db.conn.QueryRow(`
		SELECT id, name, address, description, host_type, agent_token, agent_status, last_seen, enabled, collect_stats, created_at, updated_at
		FROM hosts WHERE id = ?
	`, id).Scan(&h.ID, &h.Name, &h.Address, &h.Description, &h.HostType, &agentToken, &agentStatus, &lastSeen, &h.Enabled, &collectStats, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if agentToken.Valid {
		h.AgentToken = agentToken.String
	}
	if agentStatus.Valid {
		h.AgentStatus = agentStatus.String
	}
	if lastSeen.Valid {
		h.LastSeen = lastSeen.Time
	}
	if collectStats.Valid {
		h.CollectStats = collectStats.Bool
	} else {
		h.CollectStats = true // Default to true
	}

	return &h, nil
}

// UpdateHost updates an existing host
func (db *DB) UpdateHost(host models.Host) error {
	_, err := db.conn.Exec(`
		UPDATE hosts
		SET name = ?, address = ?, description = ?, host_type = ?, agent_token = ?, agent_status = ?, last_seen = ?, enabled = ?, collect_stats = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, host.Name, host.Address, host.Description, host.HostType, host.AgentToken, host.AgentStatus, host.LastSeen, host.Enabled, host.CollectStats, host.ID)
	return err
}

// DeleteHost deletes a host
func (db *DB) DeleteHost(id int64) error {
	_, err := db.conn.Exec("DELETE FROM hosts WHERE id = ?", id)
	return err
}

// Container operations

// SaveContainers saves a batch of containers from a scan
func (db *DB) SaveContainers(containers []models.Container) error {
	if len(containers) == 0 {
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO containers
		(id, name, image, image_id, state, status, ports, labels, created, host_id, host_name, scanned_at, networks, volumes, links, compose_project, cpu_percent, memory_usage, memory_limit, memory_percent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range containers {
		portsJSON, err := json.Marshal(c.Ports)
		if err != nil {
			return err
		}

		labelsJSON, err := json.Marshal(c.Labels)
		if err != nil {
			return err
		}

		networksJSON, err := json.Marshal(c.Networks)
		if err != nil {
			return err
		}

		volumesJSON, err := json.Marshal(c.Volumes)
		if err != nil {
			return err
		}

		linksJSON, err := json.Marshal(c.Links)
		if err != nil {
			return err
		}

		// Handle nullable stats fields
		// Store stats if memory_limit is set (indicates stats were collected)
		var cpuPercent, memoryPercent sql.NullFloat64
		var memoryUsage, memoryLimit sql.NullInt64

		if c.MemoryLimit > 0 {
			// Stats were collected - store all values including 0
			cpuPercent = sql.NullFloat64{Float64: c.CPUPercent, Valid: true}
			memoryUsage = sql.NullInt64{Int64: c.MemoryUsage, Valid: true}
			memoryLimit = sql.NullInt64{Int64: c.MemoryLimit, Valid: true}
			memoryPercent = sql.NullFloat64{Float64: c.MemoryPercent, Valid: true}
			log.Printf("DB: Saving stats for container %s (id=%s, host_id=%d, scanned_at=%v): CPU=%.2f%%, Memory=%dMB",
				c.Name, c.ID, c.HostID, c.ScannedAt, c.CPUPercent, c.MemoryUsage/1024/1024)
		}

		_, err = stmt.Exec(
			c.ID, c.Name, c.Image, c.ImageID, c.State, c.Status,
			string(portsJSON), string(labelsJSON), c.Created,
			c.HostID, c.HostName, c.ScannedAt,
			string(networksJSON), string(volumesJSON), string(linksJSON), c.ComposeProject,
			cpuPercent, memoryUsage, memoryLimit, memoryPercent,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetLatestContainers returns the most recent containers for all hosts
func (db *DB) GetLatestContainers() ([]models.Container, error) {
	query := `
		SELECT c.id, c.name, c.image, c.image_id, c.state, c.status,
		       c.ports, c.labels, c.created, c.host_id, c.host_name, c.scanned_at,
		       c.networks, c.volumes, c.links, c.compose_project,
		       c.cpu_percent, c.memory_usage, c.memory_limit, c.memory_percent
		FROM containers c
		INNER JOIN (
			SELECT host_id, MAX(scanned_at) as max_scan
			FROM containers
			GROUP BY host_id
		) latest ON c.host_id = latest.host_id AND c.scanned_at = latest.max_scan
		ORDER BY c.host_name, c.name
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return db.scanContainers(rows)
}

// GetContainersByHost returns latest containers for a specific host
func (db *DB) GetContainersByHost(hostID int64) ([]models.Container, error) {
	query := `
		SELECT c.id, c.name, c.image, c.image_id, c.state, c.status,
		       c.ports, c.labels, c.created, c.host_id, c.host_name, c.scanned_at,
		       c.networks, c.volumes, c.links, c.compose_project,
		       c.cpu_percent, c.memory_usage, c.memory_limit, c.memory_percent
		FROM containers c
		INNER JOIN (
			SELECT MAX(scanned_at) as max_scan
			FROM containers
			WHERE host_id = ?
		) latest ON c.scanned_at = latest.max_scan
		WHERE c.host_id = ?
		ORDER BY c.name
	`

	rows, err := db.conn.Query(query, hostID, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return db.scanContainers(rows)
}

// GetContainersHistory returns containers within a time range
func (db *DB) GetContainersHistory(start, end time.Time) ([]models.Container, error) {
	query := `
		SELECT id, name, image, image_id, state, status,
		       ports, labels, created, host_id, host_name, scanned_at,
		       networks, volumes, links, compose_project,
		       cpu_percent, memory_usage, memory_limit, memory_percent
		FROM containers
		WHERE scanned_at BETWEEN ? AND ?
		ORDER BY scanned_at DESC, host_name, name
	`

	rows, err := db.conn.Query(query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return db.scanContainers(rows)
}

// scanContainers helper to scan container rows
func (db *DB) scanContainers(rows *sql.Rows) ([]models.Container, error) {
	var containers []models.Container

	for rows.Next() {
		var c models.Container
		var portsJSON, labelsJSON, networksJSON, volumesJSON, linksJSON string
		var composeProject sql.NullString
		var cpuPercent, memoryPercent sql.NullFloat64
		var memoryUsage, memoryLimit sql.NullInt64

		err := rows.Scan(
			&c.ID, &c.Name, &c.Image, &c.ImageID, &c.State, &c.Status,
			&portsJSON, &labelsJSON, &c.Created,
			&c.HostID, &c.HostName, &c.ScannedAt,
			&networksJSON, &volumesJSON, &linksJSON, &composeProject,
			&cpuPercent, &memoryUsage, &memoryLimit, &memoryPercent,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(portsJSON), &c.Ports); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(labelsJSON), &c.Labels); err != nil {
			return nil, err
		}

		if networksJSON != "" && networksJSON != "null" {
			if err := json.Unmarshal([]byte(networksJSON), &c.Networks); err != nil {
				return nil, err
			}
		}

		if volumesJSON != "" && volumesJSON != "null" {
			if err := json.Unmarshal([]byte(volumesJSON), &c.Volumes); err != nil {
				return nil, err
			}
		}

		if linksJSON != "" && linksJSON != "null" {
			if err := json.Unmarshal([]byte(linksJSON), &c.Links); err != nil {
				return nil, err
			}
		}

		if composeProject.Valid {
			c.ComposeProject = composeProject.String
		}

		// Populate stats fields
		if cpuPercent.Valid {
			c.CPUPercent = cpuPercent.Float64
		}
		if memoryUsage.Valid {
			c.MemoryUsage = memoryUsage.Int64
		}
		if memoryLimit.Valid {
			c.MemoryLimit = memoryLimit.Int64
		}
		if memoryPercent.Valid {
			c.MemoryPercent = memoryPercent.Float64
		}

		containers = append(containers, c)
	}

	return containers, rows.Err()
}

// Scan result operations

// SaveScanResult saves a scan result
func (db *DB) SaveScanResult(result models.ScanResult) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO scan_results
		(host_id, host_name, started_at, completed_at, success, error, containers_found)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, result.HostID, result.HostName, result.StartedAt, result.CompletedAt,
		result.Success, result.Error, result.ContainersFound)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetScanResults returns recent scan results
func (db *DB) GetScanResults(limit int) ([]models.ScanResult, error) {
	rows, err := db.conn.Query(`
		SELECT id, host_id, host_name, started_at, completed_at, success, error, containers_found
		FROM scan_results
		ORDER BY started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ScanResult
	for rows.Next() {
		var r models.ScanResult
		var errMsg sql.NullString

		err := rows.Scan(&r.ID, &r.HostID, &r.HostName, &r.StartedAt, &r.CompletedAt,
			&r.Success, &errMsg, &r.ContainersFound)
		if err != nil {
			return nil, err
		}

		if errMsg.Valid {
			r.Error = errMsg.String
		}

		results = append(results, r)
	}

	return results, rows.Err()
}

// SaveTelemetrySubmission saves a telemetry submission record
func (db *DB) SaveTelemetrySubmission(submission *models.TelemetrySubmission) error {
	_, err := db.conn.Exec(`
		INSERT INTO telemetry_submissions (
			endpoint_name, endpoint_url, started_at, completed_at, success, error,
			hosts_count, containers_count, images_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, submission.EndpointName, submission.EndpointURL, submission.StartedAt,
		submission.CompletedAt, submission.Success, submission.Error,
		submission.HostsCount, submission.ContainersCount, submission.ImagesCount)
	return err
}

// GetTelemetrySubmissions retrieves recent telemetry submissions
func (db *DB) GetTelemetrySubmissions(limit int) ([]models.TelemetrySubmission, error) {
	rows, err := db.conn.Query(`
		SELECT id, endpoint_name, endpoint_url, started_at, completed_at, success, error,
		       hosts_count, containers_count, images_count
		FROM telemetry_submissions
		ORDER BY started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []models.TelemetrySubmission
	for rows.Next() {
		var s models.TelemetrySubmission
		var errMsg sql.NullString

		err := rows.Scan(&s.ID, &s.EndpointName, &s.EndpointURL, &s.StartedAt, &s.CompletedAt,
			&s.Success, &errMsg, &s.HostsCount, &s.ContainersCount, &s.ImagesCount)
		if err != nil {
			return nil, err
		}

		if errMsg.Valid {
			s.Error = errMsg.String
		}

		submissions = append(submissions, s)
	}

	return submissions, rows.Err()
}

// GetActivityLog retrieves unified activity log (scans + telemetry submissions)
func (db *DB) GetActivityLog(limit int, activityType string) ([]models.ActivityLogEntry, error) {
	var activities []models.ActivityLogEntry

	// Get scan results if requested
	if activityType == "all" || activityType == "scan" {
		scans, err := db.GetScanResults(limit)
		if err != nil {
			return nil, err
		}

		for _, scan := range scans {
			duration := scan.CompletedAt.Sub(scan.StartedAt).Seconds()
			details := map[string]interface{}{
				"containers_found": scan.ContainersFound,
				"host_id":          scan.HostID,
			}

			activities = append(activities, models.ActivityLogEntry{
				Type:      "scan",
				Timestamp: scan.StartedAt,
				Target:    scan.HostName,
				Duration:  duration,
				Success:   scan.Success,
				Error:     scan.Error,
				Details:   details,
			})
		}
	}

	// Get telemetry submissions if requested
	if activityType == "all" || activityType == "telemetry" {
		submissions, err := db.GetTelemetrySubmissions(limit)
		if err != nil {
			return nil, err
		}

		for _, sub := range submissions {
			duration := sub.CompletedAt.Sub(sub.StartedAt).Seconds()
			details := map[string]interface{}{
				"hosts_count":      sub.HostsCount,
				"containers_count": sub.ContainersCount,
				"images_count":     sub.ImagesCount,
				"endpoint_url":     sub.EndpointURL,
			}

			activities = append(activities, models.ActivityLogEntry{
				Type:      "telemetry",
				Timestamp: sub.StartedAt,
				Target:    sub.EndpointName,
				Duration:  duration,
				Success:   sub.Success,
				Error:     sub.Error,
				Details:   details,
			})
		}
	}

	// Sort by timestamp descending (most recent first)
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp.After(activities[j].Timestamp)
	})

	// Limit the combined results
	if len(activities) > limit {
		activities = activities[:limit]
	}

	return activities, nil
}

// CleanupOldData removes container records older than the specified duration
func (db *DB) CleanupOldData(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	_, err := db.conn.Exec("DELETE FROM containers WHERE scanned_at < ?", cutoff)
	return err
}

// Telemetry status operations

// SaveTelemetrySuccess records a successful telemetry submission
func (db *DB) SaveTelemetrySuccess(endpointName, endpointURL string) error {
	now := time.Now().UTC()
	_, err := db.conn.Exec(`
		INSERT INTO telemetry_status (endpoint_name, endpoint_url, last_success, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(endpoint_name) DO UPDATE SET
			endpoint_url = excluded.endpoint_url,
			last_success = excluded.last_success,
			updated_at = excluded.updated_at
	`, endpointName, endpointURL, now, now)
	return err
}

// SaveTelemetryFailure records a failed telemetry submission
func (db *DB) SaveTelemetryFailure(endpointName, endpointURL, reason string) error {
	now := time.Now().UTC()
	_, err := db.conn.Exec(`
		INSERT INTO telemetry_status (endpoint_name, endpoint_url, last_failure, last_failure_reason, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(endpoint_name) DO UPDATE SET
			endpoint_url = excluded.endpoint_url,
			last_failure = excluded.last_failure,
			last_failure_reason = excluded.last_failure_reason,
			updated_at = excluded.updated_at
	`, endpointName, endpointURL, now, reason, now)
	return err
}

// GetTelemetryStatus retrieves telemetry status for a specific endpoint
func (db *DB) GetTelemetryStatus(endpointName string) (*models.TelemetryEndpoint, error) {
	var status models.TelemetryEndpoint
	var lastSuccess, lastFailure sql.NullTime
	var lastFailureReason sql.NullString

	err := db.conn.QueryRow(`
		SELECT endpoint_name, endpoint_url, last_success, last_failure, last_failure_reason
		FROM telemetry_status
		WHERE endpoint_name = ?
	`, endpointName).Scan(&status.Name, &status.URL, &lastSuccess, &lastFailure, &lastFailureReason)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if lastSuccess.Valid {
		status.LastSuccess = &lastSuccess.Time
	}
	if lastFailure.Valid {
		status.LastFailure = &lastFailure.Time
	}
	if lastFailureReason.Valid {
		status.LastFailureReason = lastFailureReason.String
	}

	return &status, nil
}

// GetAllTelemetryStatuses retrieves all telemetry endpoint statuses
func (db *DB) GetAllTelemetryStatuses() (map[string]*models.TelemetryEndpoint, error) {
	rows, err := db.conn.Query(`
		SELECT endpoint_name, endpoint_url, last_success, last_failure, last_failure_reason
		FROM telemetry_status
		ORDER BY endpoint_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := make(map[string]*models.TelemetryEndpoint)
	for rows.Next() {
		var status models.TelemetryEndpoint
		var lastSuccess, lastFailure sql.NullTime
		var lastFailureReason sql.NullString

		err := rows.Scan(&status.Name, &status.URL, &lastSuccess, &lastFailure, &lastFailureReason)
		if err != nil {
			return nil, err
		}

		if lastSuccess.Valid {
			status.LastSuccess = &lastSuccess.Time
		}
		if lastFailure.Valid {
			status.LastFailure = &lastFailure.Time
		}
		if lastFailureReason.Valid {
			status.LastFailureReason = lastFailureReason.String
		}

		statuses[status.Name] = &status
	}

	return statuses, rows.Err()
}

// ClearTelemetryFailure clears the failure status for a telemetry endpoint (resets circuit breaker)
func (db *DB) ClearTelemetryFailure(endpointName string) error {
	now := time.Now().UTC()
	_, err := db.conn.Exec(`
		UPDATE telemetry_status
		SET last_failure = NULL,
		    last_failure_reason = NULL,
		    updated_at = ?
		WHERE endpoint_name = ?
	`, now, endpointName)
	return err
}

// Container lifecycle operations

// GetContainerLifecycleSummaries returns lifecycle summaries for all containers
func (db *DB) GetContainerLifecycleSummaries(limit int, hostFilter int64) ([]models.ContainerLifecycleSummary, error) {
	// Group by container name instead of ID to consolidate restarts/rebuilds
	// Use CTE with window functions for better performance
	query := `
		WITH latest_per_name AS (
			SELECT
				name,
				host_id,
				id,
				image,
				host_name,
				state,
				scanned_at,
				ROW_NUMBER() OVER (PARTITION BY name, host_id ORDER BY scanned_at DESC) as rn
			FROM containers
			WHERE (? = 0 OR host_id = ?)
		),
		host_latest AS (
			SELECT host_id, MAX(scanned_at) as max_scan
			FROM containers
			GROUP BY host_id
		)
		SELECT
			l.id,
			c.name,
			l.image,
			c.host_id,
			l.host_name,
			MIN(c.scanned_at) as first_seen,
			MAX(c.scanned_at) as last_seen,
			l.state as current_state,
			COUNT(*) as total_scans,
			COUNT(DISTINCT c.state) - 1 as state_changes,
			COUNT(DISTINCT c.image_id) - 1 as image_updates,
			0 as restart_events,
			CASE WHEN MAX(c.scanned_at) = h.max_scan THEN 1 ELSE 0 END as is_active
		FROM containers c
		INNER JOIN latest_per_name l ON c.name = l.name AND c.host_id = l.host_id AND l.rn = 1
		INNER JOIN host_latest h ON c.host_id = h.host_id
		WHERE (? = 0 OR c.host_id = ?)
		GROUP BY c.name, c.host_id, l.id, l.image, l.host_name, l.state, h.max_scan
		ORDER BY last_seen DESC
		LIMIT ?
	`

	rows, err := db.conn.Query(query, hostFilter, hostFilter, hostFilter, hostFilter, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []models.ContainerLifecycleSummary
	for rows.Next() {
		var s models.ContainerLifecycleSummary
		var isActive int
		var firstSeenStr, lastSeenStr interface{}

		err := rows.Scan(
			&s.ContainerID, &s.ContainerName, &s.Image,
			&s.HostID, &s.HostName,
			&firstSeenStr, &lastSeenStr, &s.CurrentState,
			&s.TotalScans,
			&s.StateChanges, &s.ImageUpdates, &s.RestartEvents,
			&isActive,
		)
		if err != nil {
			return nil, err
		}

		// Parse timestamps - try multiple formats
		switch v := firstSeenStr.(type) {
		case string:
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, v); err == nil {
					s.FirstSeen = t
					break
				}
			}
		case time.Time:
			s.FirstSeen = v
		}

		switch v := lastSeenStr.(type) {
		case string:
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, v); err == nil {
					s.LastSeen = t
					break
				}
			}
		case time.Time:
			s.LastSeen = v
		}

		s.IsActive = isActive == 1
		summaries = append(summaries, s)
	}

	return summaries, rows.Err()
}

// GetContainerLifecycleEvents returns detailed lifecycle events for a specific container
// Accepts container name instead of ID to show events across all container IDs with that name
func (db *DB) GetContainerLifecycleEvents(containerName string, hostID int64) ([]models.ContainerLifecycleEvent, error) {
	query := `
		SELECT
			id,
			name,
			image,
			image_id,
			state,
			scanned_at,
			LAG(state) OVER (ORDER BY scanned_at) as prev_state,
			LAG(image_id) OVER (ORDER BY scanned_at) as prev_image_id,
			LAG(image) OVER (ORDER BY scanned_at) as prev_image,
			LAG(scanned_at) OVER (ORDER BY scanned_at) as prev_scan_time
		FROM containers
		WHERE name = ? AND host_id = ?
		ORDER BY scanned_at ASC
	`

	rows, err := db.conn.Query(query, containerName, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.ContainerLifecycleEvent
	var firstSeen = true
	var lastScanTime time.Time
	var lastState string
	var totalScans int

	for rows.Next() {
		totalScans++
		var id, name, image, imageID, state string
		var scannedAtRaw interface{}
		var prevState, prevImageID, prevImage sql.NullString
		var prevScanTimeRaw sql.NullString

		err := rows.Scan(
			&id, &name, &image, &imageID, &state, &scannedAtRaw,
			&prevState, &prevImageID, &prevImage, &prevScanTimeRaw,
		)
		if err != nil {
			return nil, err
		}

		// Parse scanned_at - try multiple formats
		var scannedAt time.Time
		switch v := scannedAtRaw.(type) {
		case string:
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, v); err == nil {
					scannedAt = t
					break
				}
			}
		case time.Time:
			scannedAt = v
		}

		// Parse prev_scan_time
		var prevScanTime sql.NullTime
		if prevScanTimeRaw.Valid {
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, prevScanTimeRaw.String); err == nil {
					prevScanTime = sql.NullTime{Time: t, Valid: true}
					break
				}
			}
		}

		// First seen event
		if firstSeen {
			stateDesc := state
			if state == "running" {
				stateDesc = "running"
			} else if state == "exited" {
				stateDesc = "stopped"
			}
			events = append(events, models.ContainerLifecycleEvent{
				Timestamp:   scannedAt,
				EventType:   "first_seen",
				NewState:    state,
				NewImage:    image,
				Description: fmt.Sprintf("Container '%s' first detected (%s)", name, stateDesc),
			})
			firstSeen = false
			continue
		}

		// Detect disappearance (gap > 2 hours indicates actual downtime)
		if prevScanTime.Valid {
			gap := scannedAt.Sub(prevScanTime.Time)
			if gap > 2*time.Hour { // Only flag significant gaps (container likely removed/stopped)
				events = append(events, models.ContainerLifecycleEvent{
					Timestamp:   prevScanTime.Time,
					EventType:   "disappeared",
					OldState:    prevState.String,
					Description: fmt.Sprintf("Container disappeared (not seen for %s)", gap.Round(time.Minute)),
				})
				events = append(events, models.ContainerLifecycleEvent{
					Timestamp:   scannedAt,
					EventType:   "reappeared",
					NewState:    state,
					Description: "Container reappeared in scan",
				})
			}
		}

		// State change detected
		if prevState.Valid && prevState.String != state {
			eventType := "state_change"
			description := fmt.Sprintf("State changed from '%s' to '%s'", prevState.String, state)

			if prevState.String == "exited" && state == "running" {
				eventType = "started"
				description = "Container started"
			} else if prevState.String == "running" && state == "exited" {
				eventType = "stopped"
				description = "Container stopped"
			} else if prevState.String == "running" && state == "paused" {
				eventType = "paused"
				description = "Container paused"
			} else if prevState.String == "paused" && state == "running" {
				eventType = "resumed"
				description = "Container resumed"
			}

			events = append(events, models.ContainerLifecycleEvent{
				Timestamp:   scannedAt,
				EventType:   eventType,
				OldState:    prevState.String,
				NewState:    state,
				Description: description,
			})
		}

		// Image update detected
		if prevImageID.Valid && prevImageID.String != imageID {
			// Truncate SHAs to 12 characters for display
			oldSHA := prevImageID.String
			if len(oldSHA) > 12 {
				oldSHA = oldSHA[:12]
			}
			newSHA := imageID
			if len(newSHA) > 12 {
				newSHA = newSHA[:12]
			}

			events = append(events, models.ContainerLifecycleEvent{
				Timestamp:   scannedAt,
				EventType:   "image_updated",
				OldImage:    oldSHA,                  // Kept for backward compatibility
				NewImage:    newSHA,                  // Kept for backward compatibility
				OldImageTag: prevImage.String,        // Full image name with tag
				NewImageTag: image,                   // Full image name with tag
				OldImageSHA: oldSHA,                  // Truncated SHA
				NewImageSHA: newSHA,                  // Truncated SHA
				Description: fmt.Sprintf("Image updated to '%s'", image),
			})
		}

		// Track last scan for final event
		lastScanTime = scannedAt
		lastState = state
	}

	// Add last_seen event if we have data
	if totalScans > 0 {
		stateDesc := lastState
		if lastState == "running" {
			stateDesc = "running"
		} else if lastState == "exited" {
			stateDesc = "stopped"
		}

		events = append(events, models.ContainerLifecycleEvent{
			Timestamp:   lastScanTime,
			EventType:   "last_seen",
			NewState:    lastState,
			Description: fmt.Sprintf("Last observed (%s) - seen %d times total", stateDesc, totalScans),
		})
	}

	return events, rows.Err()
}

// CleanupRedundantScans removes redundant container scan records while preserving lifecycle milestones
// This reduces database size by keeping only: first scan, last scan, state changes, image changes, and gap indicators
func (db *DB) CleanupRedundantScans(olderThanDays int) (int, error) {
	// Strategy:
	// 1. For each container (id, host_id combination)
	// 2. Find all scans older than olderThanDays
	// 3. Keep: first scan, last scan, and any scan where state/image changed or gap > 2 hours
	// 4. Delete all redundant middle scans

	// First, find containers with redundant scans
	query := `
		WITH container_groups AS (
			SELECT
				id,
				host_id,
				COUNT(*) as scan_count,
				MIN(scanned_at) as first_scan,
				MAX(scanned_at) as last_scan
			FROM containers
			WHERE scanned_at < datetime('now', '-' || ? || ' days')
			GROUP BY id, host_id
			HAVING scan_count > 10  -- Only cleanup containers with >10 scans
		),
		scans_with_changes AS (
			SELECT
				c.rowid,
				c.id,
				c.host_id,
				c.scanned_at,
				c.state,
				c.image_id,
				LAG(c.state) OVER (PARTITION BY c.id, c.host_id ORDER BY c.scanned_at) as prev_state,
				LAG(c.image_id) OVER (PARTITION BY c.id, c.host_id ORDER BY c.scanned_at) as prev_image_id,
				LAG(c.scanned_at) OVER (PARTITION BY c.id, c.host_id ORDER BY c.scanned_at) as prev_scan_time,
				cg.first_scan,
				cg.last_scan
			FROM containers c
			INNER JOIN container_groups cg ON c.id = cg.id AND c.host_id = cg.host_id
		),
		important_scans AS (
			SELECT rowid
			FROM scans_with_changes
			WHERE
				-- Keep first scan
				scanned_at = first_scan
				-- Keep last scan
				OR scanned_at = last_scan
				-- Keep state changes
				OR (prev_state IS NOT NULL AND state != prev_state)
				-- Keep image changes
				OR (prev_image_id IS NOT NULL AND image_id != prev_image_id)
				-- Keep scans after gaps > 2 hours
				OR (prev_scan_time IS NOT NULL AND
					(julianday(scanned_at) - julianday(prev_scan_time)) * 24 > 2)
		)
		DELETE FROM containers
		WHERE rowid IN (
			SELECT sc.rowid
			FROM scans_with_changes sc
			LEFT JOIN important_scans i ON sc.rowid = i.rowid
			WHERE i.rowid IS NULL
		)
	`

	result, err := db.conn.Exec(query, olderThanDays)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup redundant scans: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(rowsAffected), nil
}

// Container stats operations

// GetContainerStats returns time-series stats for a specific container
// Combines both granular data (last hour) and aggregated data (older than 1 hour)
func (db *DB) GetContainerStats(containerID string, hostID int64, hoursBack int) ([]models.ContainerStatsPoint, error) {
	now := time.Now()
	var startTime time.Time

	if hoursBack == 0 {
		// "all" - get everything
		startTime = time.Time{} // Zero time will get all records
	} else {
		startTime = now.Add(-time.Duration(hoursBack) * time.Hour)
	}

	// Initialize as empty slice to ensure JSON returns [] instead of null when empty
	allPoints := make([]models.ContainerStatsPoint, 0)

	// Get granular data from containers table (last hour or within requested range)
	// Use LIKE to handle both short and long container IDs
	granularQuery := `
		SELECT scanned_at, cpu_percent, memory_usage, memory_limit, memory_percent
		FROM containers
		WHERE (id = ? OR id LIKE ?) AND host_id = ? AND scanned_at >= ?
		  AND (cpu_percent IS NOT NULL OR memory_usage IS NOT NULL)
		ORDER BY scanned_at ASC
	`

	// Create LIKE pattern for short ID match (first 12 chars)
	shortIDPattern := containerID[:12] + "%"
	log.Printf("GetContainerStats: Querying for containerID='%s' (or '%s'), hostID=%d, startTime=%v", containerID, shortIDPattern, hostID, startTime)
	rows, err := db.conn.Query(granularQuery, containerID, shortIDPattern, hostID, startTime)
	if err != nil {
		log.Printf("GetContainerStats: Query error: %v", err)
		return nil, err
	}
	defer rows.Close()

	rowCount := 0
	for rows.Next() {
		rowCount++
		var point models.ContainerStatsPoint
		var cpuPercent, memoryPercent sql.NullFloat64
		var memoryUsage, memoryLimit sql.NullInt64

		err := rows.Scan(&point.Timestamp, &cpuPercent, &memoryUsage, &memoryLimit, &memoryPercent)
		if err != nil {
			return nil, err
		}

		if cpuPercent.Valid {
			point.CPUPercent = cpuPercent.Float64
		}
		if memoryUsage.Valid {
			point.MemoryUsage = memoryUsage.Int64
		}
		if memoryLimit.Valid {
			point.MemoryLimit = memoryLimit.Int64
		}
		if memoryPercent.Valid {
			point.MemoryPercent = memoryPercent.Float64
		}

		allPoints = append(allPoints, point)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	log.Printf("GetContainerStats: Found %d granular data points for containerID='%s', hostID=%d", rowCount, containerID, hostID)

	// Get aggregated data if looking back more than 1 hour
	if hoursBack == 0 || hoursBack > 1 {
		aggregateQuery := `
			SELECT timestamp_hour, avg_cpu_percent, avg_memory_usage, max_memory_usage
			FROM container_stats_aggregates
			WHERE (container_id = ? OR container_id LIKE ?) AND host_id = ? AND timestamp_hour >= ?
			ORDER BY timestamp_hour ASC
		`

		aggRows, err := db.conn.Query(aggregateQuery, containerID, shortIDPattern, hostID, startTime)
		if err != nil {
			return nil, err
		}
		defer aggRows.Close()

		for aggRows.Next() {
			var point models.ContainerStatsPoint
			var avgCPU, avgMemory, maxMemory sql.NullFloat64

			err := aggRows.Scan(&point.Timestamp, &avgCPU, &avgMemory, &maxMemory)
			if err != nil {
				return nil, err
			}

			if avgCPU.Valid {
				point.CPUPercent = avgCPU.Float64
			}
			if avgMemory.Valid {
				point.MemoryUsage = int64(avgMemory.Float64)
			}

			allPoints = append(allPoints, point)
		}

		if err = aggRows.Err(); err != nil {
			return nil, err
		}
	}

	// Sort by timestamp
	sort.Slice(allPoints, func(i, j int) bool {
		return allPoints[i].Timestamp.Before(allPoints[j].Timestamp)
	})

	return allPoints, nil
}

// AggregateOldStats aggregates container stats older than 1 hour into hourly buckets
// This reduces database size while preserving historical trends
func (db *DB) AggregateOldStats() (int, error) {
	// Find the cutoff time (1 hour ago)
	cutoff := time.Now().Add(-1 * time.Hour)

	// Aggregate stats into hourly buckets
	query := `
		INSERT OR REPLACE INTO container_stats_aggregates
		(container_id, container_name, host_id, host_name, timestamp_hour, avg_cpu_percent, avg_memory_usage, max_cpu_percent, max_memory_usage, sample_count)
		SELECT
			id as container_id,
			name as container_name,
			host_id,
			host_name,
			datetime(strftime('%Y-%m-%d %H:00:00', scanned_at)) as timestamp_hour,
			AVG(cpu_percent) as avg_cpu_percent,
			AVG(memory_usage) as avg_memory_usage,
			MAX(cpu_percent) as max_cpu_percent,
			MAX(memory_usage) as max_memory_usage,
			COUNT(*) as sample_count
		FROM containers
		WHERE scanned_at < ?
		  AND (cpu_percent IS NOT NULL OR memory_usage IS NOT NULL)
		GROUP BY id, name, host_id, host_name, timestamp_hour
	`

	result, err := db.conn.Exec(query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to aggregate stats: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	// Delete the granular records that were aggregated
	deleteQuery := `
		DELETE FROM containers
		WHERE scanned_at < ?
		  AND (cpu_percent IS NOT NULL OR memory_usage IS NOT NULL)
		  AND (id, host_id, datetime(strftime('%Y-%m-%d %H:00:00', scanned_at))) IN (
			SELECT container_id, host_id, timestamp_hour
			FROM container_stats_aggregates
		)
	`

	_, err = db.conn.Exec(deleteQuery, cutoff)
	if err != nil {
		return int(rowsAffected), fmt.Errorf("failed to delete aggregated granular records: %w", err)
	}

	return int(rowsAffected), nil
}

// GetCurrentStatsForAllContainers returns the latest stats for all running containers
// Used for Prometheus /metrics endpoint
func (db *DB) GetCurrentStatsForAllContainers() ([]models.Container, error) {
	query := `
		SELECT c.id, c.name, c.image, c.host_id, c.host_name,
		       c.cpu_percent, c.memory_usage, c.memory_limit, c.memory_percent, c.state
		FROM containers c
		INNER JOIN (
			SELECT id, host_id, MAX(scanned_at) as max_scan
			FROM containers
			WHERE state = 'running'
			GROUP BY id, host_id
		) latest ON c.id = latest.id AND c.host_id = latest.host_id AND c.scanned_at = latest.max_scan
		WHERE c.state = 'running'
		  AND (c.cpu_percent IS NOT NULL OR c.memory_usage IS NOT NULL)
		ORDER BY c.host_name, c.name
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []models.Container
	for rows.Next() {
		var c models.Container
		var cpuPercent, memoryPercent sql.NullFloat64
		var memoryUsage, memoryLimit sql.NullInt64

		err := rows.Scan(
			&c.ID, &c.Name, &c.Image, &c.HostID, &c.HostName,
			&cpuPercent, &memoryUsage, &memoryLimit, &memoryPercent, &c.State,
		)
		if err != nil {
			return nil, err
		}

		if cpuPercent.Valid {
			c.CPUPercent = cpuPercent.Float64
		}
		if memoryUsage.Valid {
			c.MemoryUsage = memoryUsage.Int64
		}
		if memoryLimit.Valid {
			c.MemoryLimit = memoryLimit.Int64
		}
		if memoryPercent.Valid {
			c.MemoryPercent = memoryPercent.Float64
		}

		containers = append(containers, c)
	}

	return containers, rows.Err()
}

// parseTimestamp parses various timestamp formats from SQLite
func parseTimestamp(timestampStr string) (time.Time, error) {
	// Try various formats that SQLite might use
	formats := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339Nano,
		time.RFC3339,
	}

	var lastErr error
	for _, format := range formats {
		t, err := time.Parse(format, timestampStr)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}

	return time.Time{}, lastErr
}

// GetChangesReport generates a comprehensive environment change report for a time period
func (db *DB) GetChangesReport(start, end time.Time, hostFilter int64) (*models.ChangesReport, error) {
	report := &models.ChangesReport{
		Period: models.ReportPeriod{
			Start:         start,
			End:           end,
			DurationHours: int(end.Sub(start).Hours()),
		},
		NewContainers:     make([]models.ContainerChange, 0),
		RemovedContainers: make([]models.ContainerChange, 0),
		ImageUpdates:      make([]models.ImageUpdateChange, 0),
		StateChanges:      make([]models.StateChange, 0),
		TopRestarted:      make([]models.RestartSummary, 0),
	}

	// Build WHERE clause for host filtering
	hostFilterClause := ""
	hostFilterArgs := []interface{}{start, end}
	if hostFilter > 0 {
		hostFilterClause = " AND c.host_id = ?"
		hostFilterArgs = append(hostFilterArgs, hostFilter)
	}

	// 1. Query for new containers (first seen in period)
	// Note: We group by NAME to detect when a container name first appeared,
	// not by ID since containers get new IDs on recreation.
	// Only includes containers from enabled hosts.
	newContainersQuery := `
		WITH first_appearances AS (
			SELECT
				c.name as container_name,
				c.host_id,
				c.host_name,
				MIN(c.scanned_at) as first_seen
			FROM containers c
			INNER JOIN hosts h ON c.host_id = h.id
			WHERE h.enabled = 1` + hostFilterClause + `
			GROUP BY c.name, c.host_id, c.host_name
		),
		latest_state AS (
			SELECT
				c.id as container_id,
				c.name as container_name,
				c.image,
				c.state,
				c.host_id,
				c.scanned_at,
				ROW_NUMBER() OVER (PARTITION BY c.name, c.host_id ORDER BY c.scanned_at DESC) as rn
			FROM containers c
			INNER JOIN first_appearances f ON c.name = f.container_name AND c.host_id = f.host_id
			WHERE c.scanned_at >= f.first_seen
		)
		SELECT ls.container_id, ls.container_name, ls.image, f.host_id, f.host_name, f.first_seen, ls.state
		FROM first_appearances f
		INNER JOIN latest_state ls ON f.container_name = ls.container_name AND f.host_id = ls.host_id
		WHERE f.first_seen BETWEEN ? AND ?
		  AND ls.rn = 1
		ORDER BY f.first_seen DESC
		LIMIT 100
	`

	rows, err := db.conn.Query(newContainersQuery, append([]interface{}{start, end}, hostFilterArgs[2:]...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to query new containers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c models.ContainerChange
		var timestampStr string
		if err := rows.Scan(&c.ContainerID, &c.ContainerName, &c.Image, &c.HostID, &c.HostName, &timestampStr, &c.State); err != nil {
			return nil, err
		}
		// Parse timestamp
		c.Timestamp, err = parseTimestamp(timestampStr)
		if err != nil {
			log.Printf("Warning: failed to parse timestamp '%s': %v", timestampStr, err)
		}
		report.NewContainers = append(report.NewContainers, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// 2. Query for removed containers (seen during period, but not present at period end)
	// Note: Group by NAME to show containers that disappeared, regardless of ID changes
	// A container is "removed" if:
	//   - It was seen at least once BEFORE the period end
	//   - It is NOT seen at or after the period end (currently missing)
	// Only includes containers from enabled hosts.
	removedContainersQuery := `
		WITH last_appearances AS (
			SELECT
				c.name as container_name,
				c.host_id,
				c.host_name,
				MAX(c.scanned_at) as last_seen
			FROM containers c
			INNER JOIN hosts h ON c.host_id = h.id
			WHERE h.enabled = 1` + hostFilterClause + `
			GROUP BY c.name, c.host_id, c.host_name
		),
		final_state AS (
			SELECT
				c.id as container_id,
				c.name as container_name,
				c.image,
				c.state,
				c.host_id,
				c.scanned_at,
				ROW_NUMBER() OVER (PARTITION BY c.name, c.host_id ORDER BY c.scanned_at DESC) as rn
			FROM containers c
			INNER JOIN last_appearances l ON c.name = l.container_name AND c.host_id = l.host_id
			WHERE c.scanned_at = l.last_seen
		)
		SELECT fs.container_id, fs.container_name, fs.image, l.host_id, l.host_name, l.last_seen, fs.state
		FROM last_appearances l
		INNER JOIN final_state fs ON l.container_name = fs.container_name AND l.host_id = fs.host_id
		WHERE l.last_seen < ?
		  AND NOT EXISTS (
			  SELECT 1 FROM containers c2
			  WHERE c2.name = l.container_name
				AND c2.host_id = l.host_id
				AND c2.scanned_at >= ?
		  )
		  AND fs.rn = 1
		ORDER BY l.last_seen DESC
		LIMIT 100
	`

	rows, err = db.conn.Query(removedContainersQuery, append([]interface{}{end, end}, hostFilterArgs[2:]...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to query removed containers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c models.ContainerChange
		var timestampStr string
		if err := rows.Scan(&c.ContainerID, &c.ContainerName, &c.Image, &c.HostID, &c.HostName, &timestampStr, &c.State); err != nil {
			return nil, err
		}
		// Parse timestamp
		c.Timestamp, err = parseTimestamp(timestampStr)
		if err != nil {
			log.Printf("Warning: failed to parse timestamp '%s': %v", timestampStr, err)
		}
		report.RemovedContainers = append(report.RemovedContainers, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// 3. Query for image updates (using LAG window function)
	// Note: We partition by container NAME, not ID, because containers get new IDs when recreated.
	// This detects when a container with the same name is recreated with a different image.
	// Only includes containers from enabled hosts.
	imageUpdatesQuery := `
		WITH image_changes AS (
			SELECT
				c.id as container_id,
				c.name as container_name,
				c.host_id,
				c.host_name,
				c.image,
				c.image_id,
				c.scanned_at,
				LAG(c.image) OVER (PARTITION BY c.name, c.host_id ORDER BY c.scanned_at) as prev_image,
				LAG(c.image_id) OVER (PARTITION BY c.name, c.host_id ORDER BY c.scanned_at) as prev_image_id
			FROM containers c
			INNER JOIN hosts h ON c.host_id = h.id
			WHERE h.enabled = 1` + hostFilterClause + `
		)
		SELECT container_id, container_name, host_id, host_name,
		       prev_image, image, prev_image_id, image_id, scanned_at
		FROM image_changes
		WHERE prev_image_id IS NOT NULL
		  AND image_id != prev_image_id
		  AND scanned_at BETWEEN ? AND ?
		ORDER BY scanned_at DESC
		LIMIT 100
	`

	// Build args for image updates query: [hostFilter (if any), start, end]
	imageUpdateArgs := []interface{}{}
	if hostFilter > 0 {
		imageUpdateArgs = append(imageUpdateArgs, hostFilter)
	}
	imageUpdateArgs = append(imageUpdateArgs, start, end)

	rows, err = db.conn.Query(imageUpdatesQuery, imageUpdateArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query image updates: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var u models.ImageUpdateChange
		var timestampStr string
		if err := rows.Scan(&u.ContainerID, &u.ContainerName, &u.HostID, &u.HostName,
			&u.OldImage, &u.NewImage, &u.OldImageID, &u.NewImageID, &timestampStr); err != nil {
			return nil, err
		}
		// Parse timestamp
		u.UpdatedAt, err = parseTimestamp(timestampStr)
		if err != nil {
			log.Printf("Warning: failed to parse timestamp '%s': %v", timestampStr, err)
		}
		report.ImageUpdates = append(report.ImageUpdates, u)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// 4. Query for state changes (using LAG window function)
	// Note: We partition by container NAME, not ID, to track state across container recreations.
	// Only includes containers from enabled hosts.
	stateChangesQuery := `
		WITH state_transitions AS (
			SELECT
				c.id as container_id,
				c.name as container_name,
				c.host_id,
				c.host_name,
				c.state,
				c.scanned_at,
				LAG(c.state) OVER (PARTITION BY c.name, c.host_id ORDER BY c.scanned_at) as prev_state
			FROM containers c
			INNER JOIN hosts h ON c.host_id = h.id
			WHERE h.enabled = 1` + hostFilterClause + `
		)
		SELECT container_id, container_name, host_id, host_name,
		       prev_state, state, scanned_at
		FROM state_transitions
		WHERE prev_state IS NOT NULL
		  AND state != prev_state
		  AND scanned_at BETWEEN ? AND ?
		ORDER BY scanned_at DESC
		LIMIT 100
	`

	// Build args for state changes query: [hostFilter (if any), start, end]
	stateChangeArgs := []interface{}{}
	if hostFilter > 0 {
		stateChangeArgs = append(stateChangeArgs, hostFilter)
	}
	stateChangeArgs = append(stateChangeArgs, start, end)

	rows, err = db.conn.Query(stateChangesQuery, stateChangeArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query state changes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s models.StateChange
		var timestampStr string
		if err := rows.Scan(&s.ContainerID, &s.ContainerName, &s.HostID, &s.HostName,
			&s.OldState, &s.NewState, &timestampStr); err != nil {
			return nil, err
		}
		// Parse timestamp
		s.ChangedAt, err = parseTimestamp(timestampStr)
		if err != nil {
			log.Printf("Warning: failed to parse timestamp '%s': %v", timestampStr, err)
		}
		report.StateChanges = append(report.StateChanges, s)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// 5. Query for top restarted/active containers (counting state changes, not scans)
	// Build query dynamically based on host filter
	// Only includes containers from enabled hosts.
	// Groups by NAME to track activity across container recreations.
	var topRestartedQuery string
	if hostFilter > 0 {
		topRestartedQuery = `
			WITH state_changes AS (
				SELECT
					c.name as container_name,
					c.host_id,
					c.host_name,
					c.image,
					c.state,
					c.scanned_at,
					LAG(c.state) OVER (PARTITION BY c.name, c.host_id ORDER BY c.scanned_at) as prev_state
				FROM containers c
				INNER JOIN hosts h ON c.host_id = h.id
				WHERE c.scanned_at BETWEEN ? AND ?
				  AND c.host_id = ?
				  AND h.enabled = 1
			),
			activity_counts AS (
				SELECT
					container_name,
					host_id,
					host_name,
					MAX(image) as image,
					MAX(state) as current_state,
					COUNT(CASE WHEN prev_state IS NOT NULL AND state != prev_state THEN 1 END) as change_count
				FROM state_changes
				GROUP BY container_name, host_id, host_name
				HAVING change_count > 0
			),
			latest_container_id AS (
				SELECT
					c.name,
					c.host_id,
					MAX(c.id) as container_id
				FROM containers c
				WHERE c.scanned_at BETWEEN ? AND ?
				  AND c.host_id = ?
				GROUP BY c.name, c.host_id
			)
			SELECT
				lci.container_id,
				ac.container_name,
				ac.host_id,
				ac.host_name,
				ac.image,
				ac.change_count as restart_count,
				ac.current_state
			FROM activity_counts ac
			INNER JOIN latest_container_id lci ON ac.container_name = lci.name AND ac.host_id = lci.host_id
			ORDER BY ac.change_count DESC
			LIMIT 20
		`
	} else {
		topRestartedQuery = `
			WITH state_changes AS (
				SELECT
					c.name as container_name,
					c.host_id,
					c.host_name,
					c.image,
					c.state,
					c.scanned_at,
					LAG(c.state) OVER (PARTITION BY c.name, c.host_id ORDER BY c.scanned_at) as prev_state
				FROM containers c
				INNER JOIN hosts h ON c.host_id = h.id
				WHERE c.scanned_at BETWEEN ? AND ?
				  AND h.enabled = 1
			),
			activity_counts AS (
				SELECT
					container_name,
					host_id,
					host_name,
					MAX(image) as image,
					MAX(state) as current_state,
					COUNT(CASE WHEN prev_state IS NOT NULL AND state != prev_state THEN 1 END) as change_count
				FROM state_changes
				GROUP BY container_name, host_id, host_name
				HAVING change_count > 0
			),
			latest_container_id AS (
				SELECT
					c.name,
					c.host_id,
					MAX(c.id) as container_id
				FROM containers c
				WHERE c.scanned_at BETWEEN ? AND ?
				GROUP BY c.name, c.host_id
			)
			SELECT
				lci.container_id,
				ac.container_name,
				ac.host_id,
				ac.host_name,
				ac.image,
				ac.change_count as restart_count,
				ac.current_state
			FROM activity_counts ac
			INNER JOIN latest_container_id lci ON ac.container_name = lci.name AND ac.host_id = lci.host_id
			ORDER BY ac.change_count DESC
			LIMIT 20
		`
	}

	// Build args for query (need start/end twice plus host filter twice if applicable)
	topRestartArgs := []interface{}{start, end}
	if hostFilter > 0 {
		topRestartArgs = append(topRestartArgs, hostFilter)
	}
	topRestartArgs = append(topRestartArgs, start, end)
	if hostFilter > 0 {
		topRestartArgs = append(topRestartArgs, hostFilter)
	}

	rows, err = db.conn.Query(topRestartedQuery, topRestartArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query top restarted: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r models.RestartSummary
		if err := rows.Scan(&r.ContainerID, &r.ContainerName, &r.HostID, &r.HostName,
			&r.Image, &r.RestartCount, &r.CurrentState); err != nil {
			return nil, err
		}
		report.TopRestarted = append(report.TopRestarted, r)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// 6. Cross-check for transient containers (appeared and disappeared in same period)
	// Build a map of containers (name+host_id) that appear in both New and Removed sections
	transientMap := make(map[string]bool)

	// First pass: identify transient containers
	for _, newContainer := range report.NewContainers {
		key := fmt.Sprintf("%s-%d", newContainer.ContainerName, newContainer.HostID)
		for _, removedContainer := range report.RemovedContainers {
			removedKey := fmt.Sprintf("%s-%d", removedContainer.ContainerName, removedContainer.HostID)
			if key == removedKey {
				transientMap[key] = true
				break
			}
		}
	}

	// Second pass: mark containers as transient
	for i := range report.NewContainers {
		key := fmt.Sprintf("%s-%d", report.NewContainers[i].ContainerName, report.NewContainers[i].HostID)
		if transientMap[key] {
			report.NewContainers[i].IsTransient = true
		}
	}
	for i := range report.RemovedContainers {
		key := fmt.Sprintf("%s-%d", report.RemovedContainers[i].ContainerName, report.RemovedContainers[i].HostID)
		if transientMap[key] {
			report.RemovedContainers[i].IsTransient = true
		}
	}

	// 7. Build summary statistics
	report.Summary = models.ReportSummary{
		NewContainers:     len(report.NewContainers),
		RemovedContainers: len(report.RemovedContainers),
		ImageUpdates:      len(report.ImageUpdates),
		StateChanges:      len(report.StateChanges),
		Restarts:          len(report.TopRestarted),
	}

	// Get total hosts and containers
	hostCountQuery := `SELECT COUNT(DISTINCT host_id) FROM containers WHERE scanned_at BETWEEN ? AND ?` + hostFilterClause
	if err := db.conn.QueryRow(hostCountQuery, hostFilterArgs...).Scan(&report.Summary.TotalHosts); err != nil {
		return nil, fmt.Errorf("failed to count hosts: %w", err)
	}

	containerCountQuery := `SELECT COUNT(DISTINCT id || '-' || host_id) FROM containers WHERE scanned_at BETWEEN ? AND ?` + hostFilterClause
	if err := db.conn.QueryRow(containerCountQuery, hostFilterArgs...).Scan(&report.Summary.TotalContainers); err != nil {
		return nil, fmt.Errorf("failed to count containers: %w", err)
	}

	return report, nil
}

// GetPreference retrieves a single user preference by key
func (db *DB) GetPreference(key string) (string, error) {
	var value string
	err := db.conn.QueryRow(`SELECT value FROM user_preferences WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Return empty string if preference doesn't exist
	}
	if err != nil {
		return "", fmt.Errorf("failed to get preference: %w", err)
	}
	return value, nil
}

// SetPreference sets a user preference value (insert or update)
func (db *DB) SetPreference(key, value string) error {
	_, err := db.conn.Exec(`
		INSERT INTO user_preferences (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP
	`, key, value, value)
	if err != nil {
		return fmt.Errorf("failed to set preference: %w", err)
	}
	return nil
}

// GetAllPreferences retrieves all user preferences as a map
func (db *DB) GetAllPreferences() (map[string]string, error) {
	rows, err := db.conn.Query(`SELECT key, value FROM user_preferences`)
	if err != nil {
		return nil, fmt.Errorf("failed to query preferences: %w", err)
	}
	defer rows.Close()

	prefs := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan preference: %w", err)
		}
		prefs[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating preferences: %w", err)
	}

	return prefs, nil
}
