package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
		networks TEXT,
		volumes TEXT,
		links TEXT,
		compose_project TEXT,
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
		(id, name, image, image_id, state, status, ports, labels, created, host_id, host_name, scanned_at, networks, volumes, links, compose_project)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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

		_, err = stmt.Exec(
			c.ID, c.Name, c.Image, c.ImageID, c.State, c.Status,
			string(portsJSON), string(labelsJSON), c.Created,
			c.HostID, c.HostName, c.ScannedAt,
			string(networksJSON), string(volumesJSON), string(linksJSON), c.ComposeProject,
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
		       c.networks, c.volumes, c.links, c.compose_project
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
		       c.networks, c.volumes, c.links, c.compose_project
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
		       networks, volumes, links, compose_project
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

		err := rows.Scan(
			&c.ID, &c.Name, &c.Image, &c.ImageID, &c.State, &c.Status,
			&portsJSON, &labelsJSON, &c.Created,
			&c.HostID, &c.HostName, &c.ScannedAt,
			&networksJSON, &volumesJSON, &linksJSON, &composeProject,
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
