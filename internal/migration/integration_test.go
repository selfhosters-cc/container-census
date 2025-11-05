package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/container-census/container-census/internal/storage"
)

// TestIntegration_FirstRunScenario simulates the complete first-run workflow
func TestIntegration_FirstRunScenario(t *testing.T) {
	// Setup: Create a fresh environment
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "census.db")
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a realistic config.yaml
	configContent := `
database:
  path: ./data/census.db

server:
  host: 0.0.0.0
  port: 8080
  auth:
    enabled: true
    username: admin
    password: secret

scanner:
  interval_seconds: 180
  timeout_seconds: 60

telemetry:
  interval_hours: 336
  endpoints:
    - name: official-telemetry
      url: https://telemetry.container-census.com/api/ingest
      enabled: true
    - name: backup-collector
      url: https://backup.example.com/collect
      enabled: false
      api_key: backup-key-123

hosts:
  - name: local-docker
    address: unix:///var/run/docker.sock
    description: Local Docker daemon
  - name: remote-host-1
    address: agent://192.168.1.100:9876
    description: Remote host via agent
  - name: remote-host-2
    address: tcp://192.168.1.101:2375
    description: Remote Docker API

vulnerability:
  enabled: true
  auto_scan_new_images: true
  worker_pool_size: 3
  scan_timeout_minutes: 15
  cache_ttl_hours: 48
  rescan_interval_hours: 336
  cache_dir: /app/data/.trivy
  db_update_interval_hours: 12
  retention_days: 60
  detailed_retention_days: 21
  alert_on_critical: true
  alert_on_high: true
  max_queue_size: 200
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.yaml: %v", err)
	}

	// Step 1: Initialize database (simulates server startup)
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	t.Log("✓ Database initialized")

	// Step 2: Check if this is first run
	if !db.IsFirstRun() {
		t.Fatal("Expected IsFirstRun to be true for new database")
	}
	t.Log("✓ First run detected")

	// Step 3: Perform auto-migration
	err = ImportYAMLConfig(configPath, db)
	if err != nil {
		t.Fatalf("Auto-migration failed: %v", err)
	}
	t.Log("✓ YAML configuration imported to database")

	// Step 4: Verify all settings were migrated correctly
	settings, err := db.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	// Verify scanner settings
	if settings.Scanner.IntervalSeconds != 180 {
		t.Errorf("Scanner interval not migrated correctly: expected 180, got %d", settings.Scanner.IntervalSeconds)
	}
	if settings.Scanner.TimeoutSeconds != 60 {
		t.Errorf("Scanner timeout not migrated correctly: expected 60, got %d", settings.Scanner.TimeoutSeconds)
	}
	t.Log("✓ Scanner settings migrated correctly")

	// Verify telemetry settings
	if settings.Telemetry.IntervalHours != 336 {
		t.Errorf("Telemetry interval not migrated correctly: expected 336, got %d", settings.Telemetry.IntervalHours)
	}
	t.Log("✓ Telemetry settings migrated correctly")

	// Verify telemetry endpoints
	endpoints, err := db.GetTelemetryEndpoints()
	if err != nil {
		t.Fatalf("Failed to load endpoints: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(endpoints))
	}

	officialFound := false
	backupFound := false
	for _, ep := range endpoints {
		if ep.Name == "official-telemetry" {
			officialFound = true
			if !ep.Enabled {
				t.Error("Official telemetry should be enabled")
			}
		}
		if ep.Name == "backup-collector" {
			backupFound = true
			if ep.Enabled {
				t.Error("Backup collector should be disabled")
			}
			if ep.APIKey != "backup-key-123" {
				t.Errorf("Backup API key not migrated: expected 'backup-key-123', got '%s'", ep.APIKey)
			}
		}
	}
	if !officialFound || !backupFound {
		t.Error("Not all endpoints were imported")
	}
	t.Log("✓ Telemetry endpoints migrated correctly")

	// Verify hosts
	hosts, err := db.GetHosts()
	if err != nil {
		t.Fatalf("Failed to load hosts: %v", err)
	}
	if len(hosts) != 3 {
		t.Fatalf("Expected 3 hosts, got %d", len(hosts))
	}

	hostNames := make(map[string]bool)
	for _, host := range hosts {
		hostNames[host.Name] = true
		if !host.Enabled {
			t.Errorf("Host '%s' should be enabled by default", host.Name)
		}
		if !host.CollectStats {
			t.Errorf("Host '%s' should have CollectStats=true by default", host.Name)
		}
	}

	if !hostNames["local-docker"] || !hostNames["remote-host-1"] || !hostNames["remote-host-2"] {
		t.Error("Not all hosts were imported")
	}
	t.Log("✓ Hosts migrated correctly")

	// Verify migration flag
	migrated, err := db.GetPreference("config_migrated")
	if err != nil {
		t.Fatalf("Failed to get migration preference: %v", err)
	}
	if migrated != "true" {
		t.Errorf("Migration flag not set correctly: expected 'true', got '%s'", migrated)
	}
	t.Log("✓ Migration flag set")

	// Step 5: Verify IsFirstRun now returns false
	if db.IsFirstRun() {
		t.Error("Expected IsFirstRun to return false after migration")
	}
	t.Log("✓ IsFirstRun correctly returns false after migration")

	// Step 6: Verify second import attempt fails (prevents accidental overwrites)
	err = ImportYAMLConfig(configPath, db)
	if err == nil {
		t.Error("Second import should fail to prevent overwriting existing settings")
	}
	t.Log("✓ Second import correctly rejected")

	// Step 7: Verify settings can be loaded and used
	reloadedSettings, err := db.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to reload settings: %v", err)
	}
	if reloadedSettings.Scanner.IntervalSeconds != settings.Scanner.IntervalSeconds {
		t.Error("Settings changed after reload")
	}
	t.Log("✓ Settings persist across reloads")

	t.Log("\n=== Integration Test Summary ===")
	t.Log("✓ Database initialized successfully")
	t.Log("✓ First run detected correctly")
	t.Log("✓ YAML config auto-imported on first run")
	t.Log("✓ Scanner settings: interval=180s, timeout=60s")
	t.Log("✓ Telemetry settings: interval=336h (2 weeks)")
	t.Log("✓ Telemetry endpoints: 2 imported (1 enabled, 1 disabled)")
	t.Log("✓ Hosts: 3 imported, all enabled")
	t.Log("✓ Migration flag set correctly")
	t.Log("✓ Duplicate imports prevented")
	t.Log("✓ Settings persist correctly")
	t.Log("=== All checks passed ===")
}

// TestIntegration_ExportImportRoundtrip tests exporting and re-importing configuration
func TestIntegration_ExportImportRoundtrip(t *testing.T) {
	// Setup: Create database with settings
	tmpDir := t.TempDir()
	dbPath1 := filepath.Join(tmpDir, "original.db")

	db1, err := storage.New(dbPath1)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db1.Close()

	// Import initial configuration
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
scanner:
  interval_seconds: 240
  timeout_seconds: 90

telemetry:
  interval_hours: 72
  endpoints:
    - name: test-endpoint
      url: https://test.example.com
      enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if err := ImportYAMLConfig(configPath, db1); err != nil {
		t.Fatalf("Initial import failed: %v", err)
	}
	t.Log("✓ Original configuration imported")

	// Export to YAML
	exportedYAML, err := ExportToYAML(db1)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	t.Log("✓ Configuration exported to YAML")

	// Save exported YAML
	exportPath := filepath.Join(tmpDir, "exported.yaml")
	if err := os.WriteFile(exportPath, exportedYAML, 0644); err != nil {
		t.Fatalf("Failed to write exported YAML: %v", err)
	}

	// Create new database and import the exported configuration
	dbPath2 := filepath.Join(tmpDir, "imported.db")
	db2, err := storage.New(dbPath2)
	if err != nil {
		t.Fatalf("Failed to create second database: %v", err)
	}
	defer db2.Close()

	if err := ImportYAMLConfig(exportPath, db2); err != nil {
		t.Fatalf("Re-import failed: %v", err)
	}
	t.Log("✓ Exported configuration re-imported to new database")

	// Verify settings match
	settings1, err := db1.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load original settings: %v", err)
	}

	settings2, err := db2.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load re-imported settings: %v", err)
	}

	if settings1.Scanner.IntervalSeconds != settings2.Scanner.IntervalSeconds {
		t.Errorf("Scanner interval mismatch: %d != %d",
			settings1.Scanner.IntervalSeconds, settings2.Scanner.IntervalSeconds)
	}

	if settings1.Scanner.TimeoutSeconds != settings2.Scanner.TimeoutSeconds {
		t.Errorf("Scanner timeout mismatch: %d != %d",
			settings1.Scanner.TimeoutSeconds, settings2.Scanner.TimeoutSeconds)
	}

	if settings1.Telemetry.IntervalHours != settings2.Telemetry.IntervalHours {
		t.Errorf("Telemetry interval mismatch: %d != %d",
			settings1.Telemetry.IntervalHours, settings2.Telemetry.IntervalHours)
	}

	t.Log("✓ Export/Import roundtrip successful - settings match perfectly")
}

// TestIntegration_DefaultsWhenNoConfig tests behavior when config.yaml doesn't exist
func TestIntegration_DefaultsWhenNoConfig(t *testing.T) {
	// Setup: Create database with no config file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "census.db")
	nonexistentConfig := filepath.Join(tmpDir, "nonexistent.yaml")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Attempt import with missing config (should use defaults)
	err = ImportYAMLConfig(nonexistentConfig, db)
	if err != nil {
		t.Fatalf("Import should succeed with defaults when config missing: %v", err)
	}
	t.Log("✓ Import succeeded with defaults when config.yaml missing")

	// Verify defaults were loaded
	settings, err := db.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	expectedDefaults := storage.GetDefaultSettings()

	if settings.Scanner.IntervalSeconds != expectedDefaults.Scanner.IntervalSeconds {
		t.Errorf("Default interval not applied: expected %d, got %d",
			expectedDefaults.Scanner.IntervalSeconds, settings.Scanner.IntervalSeconds)
	}

	if settings.Scanner.TimeoutSeconds != expectedDefaults.Scanner.TimeoutSeconds {
		t.Errorf("Default timeout not applied: expected %d, got %d",
			expectedDefaults.Scanner.TimeoutSeconds, settings.Scanner.TimeoutSeconds)
	}

	if settings.Telemetry.IntervalHours != expectedDefaults.Telemetry.IntervalHours {
		t.Errorf("Default telemetry interval not applied: expected %d, got %d",
			expectedDefaults.Telemetry.IntervalHours, settings.Telemetry.IntervalHours)
	}

	t.Log("✓ Default settings applied correctly:")
	t.Logf("  - Scanner interval: %d seconds", settings.Scanner.IntervalSeconds)
	t.Logf("  - Scanner timeout: %d seconds", settings.Scanner.TimeoutSeconds)
	t.Logf("  - Telemetry interval: %d hours", settings.Telemetry.IntervalHours)
	t.Logf("  - Notification rate limit: %d/hour", settings.Notification.RateLimitMax)
}
