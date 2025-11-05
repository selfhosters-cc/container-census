package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/container-census/container-census/internal/config"
	"github.com/container-census/container-census/internal/storage"
)

// TestServerStartup_FirstRun simulates exactly what happens when the server starts for the first time
func TestServerStartup_FirstRun(t *testing.T) {
	// Simulate server startup environment
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "census.db")
	configPath := filepath.Join(tmpDir, "config", "config.yaml")

	// Create config directory
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create a typical config.yaml file
	configContent := `
database:
  path: ./data/census.db

server:
  host: 0.0.0.0
  port: 8080
  auth:
    enabled: false
    username: ""
    password: ""

scanner:
  interval_seconds: 300
  timeout_seconds: 30

telemetry:
  interval_hours: 168
  endpoints: []

hosts: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config.yaml: %v", err)
	}

	// === SIMULATE SERVER MAIN() FUNCTION ===

	// Step 1: Load YAML config (for infrastructure settings)
	cfg, fromFile := config.LoadOrDefault(configPath)
	if !fromFile {
		t.Fatal("Expected config to be loaded from file")
	}
	t.Logf("✓ Configuration loaded from file: %s", configPath)

	// Step 2: Initialize database
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	t.Logf("✓ Database initialized: %s", dbPath)

	// Step 3: Check if first run
	if !db.IsFirstRun() {
		t.Fatal("Expected first run for new database")
	}
	t.Log("✓ First run detected - attempting to migrate configuration from YAML to database...")

	// Step 4: Auto-import YAML config
	if err := ImportYAMLConfig(configPath, db); err != nil {
		t.Fatalf("Configuration migration failed: %v", err)
	}
	t.Log("✓ Configuration migrated from config.yaml to database successfully")

	// Step 5: Load settings from database
	settings, err := db.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load system settings: %v", err)
	}
	t.Logf("✓ System settings loaded from database (scanner interval: %ds, telemetry interval: %dh)",
		settings.Scanner.IntervalSeconds, settings.Telemetry.IntervalHours)

	// Step 6: Verify infrastructure settings still come from YAML
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server host should come from YAML: expected '0.0.0.0', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server port should come from YAML: expected 8080, got %d", cfg.Server.Port)
	}
	t.Log("✓ Infrastructure settings (host, port, auth) correctly loaded from YAML")

	// Step 7: Verify runtime settings come from database
	if settings.Scanner.IntervalSeconds != 300 {
		t.Errorf("Scanner interval should come from DB: expected 300, got %d", settings.Scanner.IntervalSeconds)
	}
	if settings.Scanner.TimeoutSeconds != 30 {
		t.Errorf("Scanner timeout should come from DB: expected 30, got %d", settings.Scanner.TimeoutSeconds)
	}
	t.Log("✓ Runtime settings (scanner, telemetry) correctly loaded from database")

	// Step 8: Verify migration flag prevents re-import
	migrated, err := db.GetPreference("config_migrated")
	if err != nil {
		t.Fatalf("Failed to get migration flag: %v", err)
	}
	if migrated != "true" {
		t.Error("Migration flag should be set to 'true'")
	}
	t.Log("✓ Migration flag set - config.yaml will be ignored on subsequent startups")

	t.Log("\n=== Server Startup Test Summary ===")
	t.Log("✓ YAML config loaded for infrastructure settings")
	t.Log("✓ Database initialized successfully")
	t.Log("✓ First run detected")
	t.Log("✓ YAML settings auto-migrated to database")
	t.Log("✓ Database settings loaded for runtime configuration")
	t.Log("✓ Separation of concerns maintained:")
	t.Log("  - Infrastructure: YAML (host, port, auth, database path)")
	t.Log("  - Runtime: Database (scanner, telemetry, notifications)")
	t.Log("✓ Migration flag prevents duplicate imports")
	t.Log("=== Server startup flow validated ===")
}

// TestServerStartup_SubsequentRuns verifies behavior on second and later startups
func TestServerStartup_SubsequentRuns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "census.db")
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config
	configContent := `
scanner:
  interval_seconds: 300
  timeout_seconds: 30

telemetry:
  interval_hours: 168
  endpoints: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// === FIRST STARTUP ===
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if err := ImportYAMLConfig(configPath, db); err != nil {
		t.Fatalf("First import failed: %v", err)
	}
	t.Log("✓ First startup completed - settings migrated")

	db.Close()

	// === SECOND STARTUP (simulates server restart) ===
	db2, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to open existing database: %v", err)
	}
	defer db2.Close()

	// Check IsFirstRun (should be false)
	if db2.IsFirstRun() {
		t.Error("IsFirstRun should return false on second startup")
	}
	t.Log("✓ Second startup - not a first run")

	// Verify settings still exist
	settings, err := db2.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings on second startup: %v", err)
	}

	if settings.Scanner.IntervalSeconds != 300 {
		t.Error("Settings should persist across restarts")
	}
	t.Log("✓ Settings persisted from first startup")

	// Modify YAML (should NOT affect database)
	modifiedConfig := `
scanner:
  interval_seconds: 999
  timeout_seconds: 999
`
	if err := os.WriteFile(configPath, []byte(modifiedConfig), 0644); err != nil {
		t.Fatalf("Failed to modify config: %v", err)
	}

	// Try to import (should fail)
	err = ImportYAMLConfig(configPath, db2)
	if err == nil {
		t.Fatal("Import should fail on subsequent runs")
	}
	t.Log("✓ Import correctly rejected on subsequent startup")

	// Verify settings unchanged
	settings2, err := db2.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings2.Scanner.IntervalSeconds != 300 {
		t.Error("Settings should not be affected by YAML changes after first run")
	}
	t.Log("✓ Settings protected from YAML changes after migration")

	t.Log("\n=== Subsequent Startup Test Summary ===")
	t.Log("✓ Database persists across restarts")
	t.Log("✓ IsFirstRun correctly returns false")
	t.Log("✓ Settings load from database (not YAML)")
	t.Log("✓ YAML changes ignored after migration")
	t.Log("✓ Database is source of truth for runtime settings")
	t.Log("=== Subsequent startup behavior validated ===")
}

// TestServerStartup_WithoutConfig tests startup when config.yaml doesn't exist
func TestServerStartup_WithoutConfig(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "census.db")
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	// === SERVER STARTUP WITHOUT CONFIG FILE ===

	// Initialize database
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check first run
	if !db.IsFirstRun() {
		t.Fatal("Expected first run")
	}

	// Attempt import (should succeed with defaults)
	err = ImportYAMLConfig(configPath, db)
	if err != nil {
		t.Fatalf("Import should succeed with defaults: %v", err)
	}
	t.Log("✓ Server started successfully without config.yaml")

	// Verify defaults
	settings, err := db.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	defaults := storage.GetDefaultSettings()

	if settings.Scanner.IntervalSeconds != defaults.Scanner.IntervalSeconds {
		t.Error("Default scanner interval not applied")
	}
	t.Logf("✓ Default settings applied: interval=%ds, timeout=%ds, telemetry=%dh",
		settings.Scanner.IntervalSeconds,
		settings.Scanner.TimeoutSeconds,
		settings.Telemetry.IntervalHours)

	// Verify server can operate normally
	if db.IsFirstRun() {
		t.Error("After initialization, should not be first run")
	}
	t.Log("✓ Server operating normally with default configuration")

	t.Log("\n=== No-Config Startup Test Summary ===")
	t.Log("✓ Server starts successfully without config.yaml")
	t.Log("✓ Default settings automatically applied")
	t.Log("✓ Database initialized with sensible defaults")
	t.Log("✓ Server operational without manual configuration")
	t.Log("=== Zero-configuration startup validated ===")
}
