package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/container-census/container-census/internal/vulnerability"
)

// GetVulnerabilityScan retrieves a vulnerability scan by image ID
func (db *DB) GetVulnerabilityScan(imageID string) (*vulnerability.VulnerabilityScan, error) {
	query := `
		SELECT id, image_id, image_name, scanned_at, scan_duration_ms, success,
		       error, trivy_db_version, total_vulnerabilities,
		       critical_count, high_count, medium_count, low_count, unknown_count
		FROM vulnerability_scans
		WHERE image_id = ?
		ORDER BY scanned_at DESC
		LIMIT 1
	`

	var scan vulnerability.VulnerabilityScan
	var errorText sql.NullString

	err := db.conn.QueryRow(query, imageID).Scan(
		&scan.ID,
		&scan.ImageID,
		&scan.ImageName,
		&scan.ScannedAt,
		&scan.ScanDurationMs,
		&scan.Success,
		&errorText,
		&scan.TrivyDBVersion,
		&scan.TotalVulnerabilities,
		&scan.SeverityCounts.Critical,
		&scan.SeverityCounts.High,
		&scan.SeverityCounts.Medium,
		&scan.SeverityCounts.Low,
		&scan.SeverityCounts.Unknown,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get vulnerability scan: %w", err)
	}

	if errorText.Valid {
		scan.Error = errorText.String
	}

	return &scan, nil
}

// SaveVulnerabilityScan saves or updates a vulnerability scan
func (db *DB) SaveVulnerabilityScan(scan *vulnerability.VulnerabilityScan, vulnerabilities []vulnerability.Vulnerability) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert or replace scan record
	scanQuery := `
		INSERT OR REPLACE INTO vulnerability_scans
		(image_id, image_name, scanned_at, scan_duration_ms, success, error,
		 trivy_db_version, total_vulnerabilities, critical_count, high_count,
		 medium_count, low_count, unknown_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	errorText := sql.NullString{}
	if scan.Error != "" {
		errorText.Valid = true
		errorText.String = scan.Error
	}

	result, err := tx.Exec(scanQuery,
		scan.ImageID,
		scan.ImageName,
		scan.ScannedAt,
		scan.ScanDurationMs,
		scan.Success,
		errorText,
		scan.TrivyDBVersion,
		scan.TotalVulnerabilities,
		scan.SeverityCounts.Critical,
		scan.SeverityCounts.High,
		scan.SeverityCounts.Medium,
		scan.SeverityCounts.Low,
		scan.SeverityCounts.Unknown,
	)
	if err != nil {
		return fmt.Errorf("failed to save vulnerability scan: %w", err)
	}

	// Get the scan ID
	scanID, err := result.LastInsertId()
	if err != nil {
		// If we're replacing, get the existing ID
		err = tx.QueryRow("SELECT id FROM vulnerability_scans WHERE image_id = ?", scan.ImageID).Scan(&scanID)
		if err != nil {
			return fmt.Errorf("failed to get scan ID: %w", err)
		}
	}

	// Delete old vulnerabilities for this image
	_, err = tx.Exec("DELETE FROM vulnerabilities WHERE image_id = ?", scan.ImageID)
	if err != nil {
		return fmt.Errorf("failed to delete old vulnerabilities: %w", err)
	}

	// Insert new vulnerabilities
	if len(vulnerabilities) > 0 {
		vulnQuery := `
			INSERT INTO vulnerabilities
			(image_id, vulnerability_id, pkg_name, installed_version, fixed_version,
			 severity, title, description, published_date, last_modified_date, primary_url)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		stmt, err := tx.Prepare(vulnQuery)
		if err != nil {
			return fmt.Errorf("failed to prepare vulnerability insert: %w", err)
		}
		defer stmt.Close()

		for _, vuln := range vulnerabilities {
			_, err = stmt.Exec(
				vuln.ImageID,
				vuln.VulnerabilityID,
				vuln.PkgName,
				vuln.InstalledVersion,
				vuln.FixedVersion,
				vuln.Severity,
				vuln.Title,
				vuln.Description,
				vuln.PublishedDate,
				vuln.LastModifiedDate,
				vuln.PrimaryURL,
			)
			if err != nil {
				return fmt.Errorf("failed to insert vulnerability: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	scan.ID = int(scanID)
	return nil
}

// GetVulnerabilities retrieves all vulnerabilities for an image
func (db *DB) GetVulnerabilities(imageID string) ([]vulnerability.Vulnerability, error) {
	query := `
		SELECT id, image_id, vulnerability_id, pkg_name, installed_version, fixed_version,
		       severity, title, description, published_date, last_modified_date, primary_url
		FROM vulnerabilities
		WHERE image_id = ?
		ORDER BY
			CASE severity
				WHEN 'CRITICAL' THEN 1
				WHEN 'HIGH' THEN 2
				WHEN 'MEDIUM' THEN 3
				WHEN 'LOW' THEN 4
				ELSE 5
			END,
			vulnerability_id
	`

	rows, err := db.conn.Query(query, imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query vulnerabilities: %w", err)
	}
	defer rows.Close()

	vulns := make([]vulnerability.Vulnerability, 0)
	for rows.Next() {
		var vuln vulnerability.Vulnerability
		err := rows.Scan(
			&vuln.ID,
			&vuln.ImageID,
			&vuln.VulnerabilityID,
			&vuln.PkgName,
			&vuln.InstalledVersion,
			&vuln.FixedVersion,
			&vuln.Severity,
			&vuln.Title,
			&vuln.Description,
			&vuln.PublishedDate,
			&vuln.LastModifiedDate,
			&vuln.PrimaryURL,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan vulnerability: %w", err)
		}
		vulns = append(vulns, vuln)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vulnerabilities: %w", err)
	}

	return vulns, nil
}

// GetVulnerabilitySummary returns an overview of all vulnerability scans
func (db *DB) GetVulnerabilitySummary() (*vulnerability.ScanSummary, error) {
	query := `
		SELECT
			COUNT(*) as total_scanned,
			COUNT(CASE WHEN total_vulnerabilities > 0 THEN 1 END) as with_vulns,
			COALESCE(SUM(total_vulnerabilities), 0) as total_vulns,
			COALESCE(SUM(critical_count), 0) as total_critical,
			COALESCE(SUM(high_count), 0) as total_high,
			COALESCE(SUM(medium_count), 0) as total_medium,
			COALESCE(SUM(low_count), 0) as total_low,
			COALESCE(SUM(unknown_count), 0) as total_unknown,
			MAX(scanned_at) as last_scan
		FROM vulnerability_scans
		WHERE success = 1
	`

	var summary vulnerability.ScanSummary
	var lastScanStr sql.NullString

	err := db.conn.QueryRow(query).Scan(
		&summary.TotalImagesScanned,
		&summary.ImagesWithVulnerabilities,
		&summary.TotalVulnerabilities,
		&summary.SeverityCounts.Critical,
		&summary.SeverityCounts.High,
		&summary.SeverityCounts.Medium,
		&summary.SeverityCounts.Low,
		&summary.SeverityCounts.Unknown,
		&lastScanStr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get vulnerability summary: %w", err)
	}

	// Parse last scan time manually (SQLite stores as string)
	if lastScanStr.Valid && lastScanStr.String != "" {
		// Try parsing common SQLite timestamp formats
		formats := []string{
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05.999999999-07:00",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, lastScanStr.String); err == nil {
				summary.LastScan = t
				break
			}
		}
	}

	// Get Trivy DB version from most recent scan
	var dbVersion sql.NullString
	err = db.conn.QueryRow("SELECT trivy_db_version FROM vulnerability_scans WHERE success = 1 ORDER BY scanned_at DESC LIMIT 1").Scan(&dbVersion)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get trivy db version: %w", err)
	}
	if dbVersion.Valid {
		summary.TrivyDBVersion = dbVersion.String
	}

	return &summary, nil
}

// GetAllVulnerabilityScans returns all vulnerability scans with optional filters
func (db *DB) GetAllVulnerabilityScans(limit int) ([]vulnerability.VulnerabilityScan, error) {
	query := `
		SELECT id, image_id, image_name, scanned_at, scan_duration_ms, success,
		       error, trivy_db_version, total_vulnerabilities,
		       critical_count, high_count, medium_count, low_count, unknown_count
		FROM vulnerability_scans
		ORDER BY scanned_at DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query vulnerability scans: %w", err)
	}
	defer rows.Close()

	scans := make([]vulnerability.VulnerabilityScan, 0)
	for rows.Next() {
		var scan vulnerability.VulnerabilityScan
		var errorText sql.NullString

		err := rows.Scan(
			&scan.ID,
			&scan.ImageID,
			&scan.ImageName,
			&scan.ScannedAt,
			&scan.ScanDurationMs,
			&scan.Success,
			&errorText,
			&scan.TrivyDBVersion,
			&scan.TotalVulnerabilities,
			&scan.SeverityCounts.Critical,
			&scan.SeverityCounts.High,
			&scan.SeverityCounts.Medium,
			&scan.SeverityCounts.Low,
			&scan.SeverityCounts.Unknown,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan vulnerability scan: %w", err)
		}

		if errorText.Valid {
			scan.Error = errorText.String
		}

		scans = append(scans, scan)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vulnerability scans: %w", err)
	}

	return scans, nil
}

// UpdateImageContainer updates the image-to-container mapping
func (db *DB) UpdateImageContainer(imageID, containerID string, hostID int) error {
	query := `
		INSERT OR REPLACE INTO image_containers (image_id, container_id, host_id, last_seen)
		VALUES (?, ?, ?, ?)
	`

	_, err := db.conn.Exec(query, imageID, containerID, hostID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update image container mapping: %w", err)
	}

	return nil
}

// GetContainersForImage returns all containers using a specific image
func (db *DB) GetContainersForImage(imageID string) ([]struct {
	ContainerID string
	HostID      int
}, error) {
	query := `
		SELECT DISTINCT container_id, host_id
		FROM image_containers
		WHERE image_id = ?
		AND last_seen > datetime('now', '-1 hour')
	`

	rows, err := db.conn.Query(query, imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query containers for image: %w", err)
	}
	defer rows.Close()

	containers := make([]struct {
		ContainerID string
		HostID      int
	}, 0)

	for rows.Next() {
		var c struct {
			ContainerID string
			HostID      int
		}
		if err := rows.Scan(&c.ContainerID, &c.HostID); err != nil {
			return nil, fmt.Errorf("failed to scan container: %w", err)
		}
		containers = append(containers, c)
	}

	return containers, nil
}

// CleanupOldVulnerabilityData removes old vulnerability scans and details
func (db *DB) CleanupOldVulnerabilityData(retentionDays, detailedRetentionDays int) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete detailed vulnerabilities older than detailedRetentionDays
	_, err = tx.Exec(`
		DELETE FROM vulnerabilities
		WHERE image_id IN (
			SELECT image_id FROM vulnerability_scans
			WHERE scanned_at < datetime('now', '-' || ? || ' days')
		)
	`, detailedRetentionDays)
	if err != nil {
		return fmt.Errorf("failed to cleanup old vulnerabilities: %w", err)
	}

	// Delete old scans older than retentionDays
	_, err = tx.Exec(`
		DELETE FROM vulnerability_scans
		WHERE scanned_at < datetime('now', '-' || ? || ' days')
	`, retentionDays)
	if err != nil {
		return fmt.Errorf("failed to cleanup old vulnerability scans: %w", err)
	}

	// Cleanup old image container mappings
	_, err = tx.Exec(`
		DELETE FROM image_containers
		WHERE last_seen < datetime('now', '-7 days')
	`)
	if err != nil {
		return fmt.Errorf("failed to cleanup old image containers: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit cleanup transaction: %w", err)
	}

	return nil
}

// GetVulnerabilitySettings retrieves vulnerability scanner settings from database
func (db *DB) GetVulnerabilitySettings() (map[string]string, error) {
	query := "SELECT key, value FROM vulnerability_settings"

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query vulnerability settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings[key] = value
	}

	return settings, nil
}

// SaveVulnerabilitySettings saves vulnerability scanner settings to database
func (db *DB) SaveVulnerabilitySettings(config *vulnerability.Config) error {
	// Convert config to JSON for storage
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT OR REPLACE INTO vulnerability_settings (key, value, updated_at)
		VALUES ('config', ?, ?)
	`

	_, err = db.conn.Exec(query, string(configJSON), time.Now())
	if err != nil {
		return fmt.Errorf("failed to save vulnerability settings: %w", err)
	}

	return nil
}

// LoadVulnerabilitySettings loads vulnerability scanner settings from database
func (db *DB) LoadVulnerabilitySettings() (*vulnerability.Config, error) {
	var configJSON string
	query := "SELECT value FROM vulnerability_settings WHERE key = 'config' LIMIT 1"

	err := db.conn.QueryRow(query).Scan(&configJSON)
	if err == sql.ErrNoRows {
		// No settings saved, return default config
		return vulnerability.DefaultConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load vulnerability settings: %w", err)
	}

	var config vulnerability.Config
	err = json.Unmarshal([]byte(configJSON), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
