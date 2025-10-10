package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

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
		PRIMARY KEY (id, host_id, scanned_at),
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_containers_host_id ON containers(host_id);
	CREATE INDEX IF NOT EXISTS idx_containers_scanned_at ON containers(scanned_at);
	CREATE INDEX IF NOT EXISTS idx_containers_state ON containers(state);

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

// Host operations

// AddHost adds a new host
func (db *DB) AddHost(host models.Host) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO hosts (name, address, description, host_type, agent_token, agent_status, last_seen, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		host.Name, host.Address, host.Description, host.HostType, host.AgentToken, host.AgentStatus, host.LastSeen, host.Enabled,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetHosts returns all hosts
func (db *DB) GetHosts() ([]models.Host, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, address, description, host_type, agent_token, agent_status, last_seen, enabled, created_at, updated_at
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

		if err := rows.Scan(&h.ID, &h.Name, &h.Address, &h.Description, &h.HostType, &agentToken, &agentStatus, &lastSeen, &h.Enabled, &h.CreatedAt, &h.UpdatedAt); err != nil {
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

		hosts = append(hosts, h)
	}

	return hosts, rows.Err()
}

// GetHost returns a single host by ID
func (db *DB) GetHost(id int64) (*models.Host, error) {
	var h models.Host
	var lastSeen sql.NullTime
	var agentToken, agentStatus sql.NullString

	err := db.conn.QueryRow(`
		SELECT id, name, address, description, host_type, agent_token, agent_status, last_seen, enabled, created_at, updated_at
		FROM hosts WHERE id = ?
	`, id).Scan(&h.ID, &h.Name, &h.Address, &h.Description, &h.HostType, &agentToken, &agentStatus, &lastSeen, &h.Enabled, &h.CreatedAt, &h.UpdatedAt)
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

	return &h, nil
}

// UpdateHost updates an existing host
func (db *DB) UpdateHost(host models.Host) error {
	_, err := db.conn.Exec(`
		UPDATE hosts
		SET name = ?, address = ?, description = ?, host_type = ?, agent_token = ?, agent_status = ?, last_seen = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, host.Name, host.Address, host.Description, host.HostType, host.AgentToken, host.AgentStatus, host.LastSeen, host.Enabled, host.ID)
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
		(id, name, image, image_id, state, status, ports, labels, created, host_id, host_name, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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

		_, err = stmt.Exec(
			c.ID, c.Name, c.Image, c.ImageID, c.State, c.Status,
			string(portsJSON), string(labelsJSON), c.Created,
			c.HostID, c.HostName, c.ScannedAt,
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
		       c.ports, c.labels, c.created, c.host_id, c.host_name, c.scanned_at
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
		       c.ports, c.labels, c.created, c.host_id, c.host_name, c.scanned_at
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
		       ports, labels, created, host_id, host_name, scanned_at
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
		var portsJSON, labelsJSON string

		err := rows.Scan(
			&c.ID, &c.Name, &c.Image, &c.ImageID, &c.State, &c.Status,
			&portsJSON, &labelsJSON, &c.Created,
			&c.HostID, &c.HostName, &c.ScannedAt,
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

// CleanupOldData removes container records older than the specified duration
func (db *DB) CleanupOldData(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	_, err := db.conn.Exec("DELETE FROM containers WHERE scanned_at < ?", cutoff)
	return err
}
