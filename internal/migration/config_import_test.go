package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
)

func TestImportYAMLConfig_FirstRun_WithValidYAML(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test YAML config
	yamlContent := `
scanner:
  interval_seconds: 120
  timeout_seconds: 45

telemetry:
  interval_hours: 24
  endpoints:
    - name: test-endpoint
      url: https://example.com/telemetry
      enabled: true
      api_key: test-key-123

hosts:
  - name: test-host
    address: unix:///var/run/docker.sock
    description: Test host for unit tests
`
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	// Test auto-import
	if !db.IsFirstRun() {
		t.Fatal("Expected IsFirstRun to be true for new database")
	}

	err = ImportYAMLConfig(yamlPath, db)
	if err != nil {
		t.Fatalf("ImportYAMLConfig failed: %v", err)
	}

	// Verify settings were imported
	settings, err := db.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Scanner.IntervalSeconds != 120 {
		t.Errorf("Expected interval_seconds=120, got %d", settings.Scanner.IntervalSeconds)
	}

	if settings.Scanner.TimeoutSeconds != 45 {
		t.Errorf("Expected timeout_seconds=45, got %d", settings.Scanner.TimeoutSeconds)
	}

	if settings.Telemetry.IntervalHours != 24 {
		t.Errorf("Expected interval_hours=24, got %d", settings.Telemetry.IntervalHours)
	}

	// Verify telemetry endpoints were imported
	endpoints, err := db.GetTelemetryEndpoints()
	if err != nil {
		t.Fatalf("Failed to load telemetry endpoints: %v", err)
	}

	if len(endpoints) != 1 {
		t.Fatalf("Expected 1 endpoint, got %d", len(endpoints))
	}

	if endpoints[0].Name != "test-endpoint" {
		t.Errorf("Expected endpoint name 'test-endpoint', got '%s'", endpoints[0].Name)
	}

	if endpoints[0].URL != "https://example.com/telemetry" {
		t.Errorf("Expected endpoint URL 'https://example.com/telemetry', got '%s'", endpoints[0].URL)
	}

	if !endpoints[0].Enabled {
		t.Error("Expected endpoint to be enabled")
	}

	if endpoints[0].APIKey != "test-key-123" {
		t.Errorf("Expected API key 'test-key-123', got '%s'", endpoints[0].APIKey)
	}

	// Verify hosts were imported
	hosts, err := db.GetHosts()
	if err != nil {
		t.Fatalf("Failed to load hosts: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(hosts))
	}

	if hosts[0].Name != "test-host" {
		t.Errorf("Expected host name 'test-host', got '%s'", hosts[0].Name)
	}

	if hosts[0].Address != "unix:///var/run/docker.sock" {
		t.Errorf("Expected host address 'unix:///var/run/docker.sock', got '%s'", hosts[0].Address)
	}

	if hosts[0].Description != "Test host for unit tests" {
		t.Errorf("Expected host description 'Test host for unit tests', got '%s'", hosts[0].Description)
	}

	// Verify migration flag was set
	migrated, err := db.GetPreference("config_migrated")
	if err != nil {
		t.Fatalf("Failed to get config_migrated preference: %v", err)
	}

	if migrated != "true" {
		t.Errorf("Expected config_migrated='true', got '%s'", migrated)
	}

	// Verify IsFirstRun now returns false
	if db.IsFirstRun() {
		t.Error("Expected IsFirstRun to be false after import")
	}
}

func TestImportYAMLConfig_FirstRun_WithMissingYAML(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test auto-import with missing YAML (should use defaults)
	yamlPath := filepath.Join(tmpDir, "nonexistent.yaml")

	err = ImportYAMLConfig(yamlPath, db)
	if err != nil {
		t.Fatalf("ImportYAMLConfig should not fail with missing YAML: %v", err)
	}

	// Verify default settings were loaded
	settings, err := db.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	// Check defaults
	if settings.Scanner.IntervalSeconds != 300 {
		t.Errorf("Expected default interval_seconds=300, got %d", settings.Scanner.IntervalSeconds)
	}

	if settings.Scanner.TimeoutSeconds != 30 {
		t.Errorf("Expected default timeout_seconds=30, got %d", settings.Scanner.TimeoutSeconds)
	}

	if settings.Telemetry.IntervalHours != 168 {
		t.Errorf("Expected default interval_hours=168, got %d", settings.Telemetry.IntervalHours)
	}

	// Verify no hosts were imported
	hosts, err := db.GetHosts()
	if err != nil {
		t.Fatalf("Failed to load hosts: %v", err)
	}

	if len(hosts) != 0 {
		t.Errorf("Expected 0 hosts with missing YAML, got %d", len(hosts))
	}
}

func TestImportYAMLConfig_NotFirstRun(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Save some settings to make it not a first run
	defaults := storage.GetDefaultSettings()
	if err := db.SaveSystemSettings(defaults); err != nil {
		t.Fatalf("Failed to save initial settings: %v", err)
	}

	// Create test YAML config
	yamlContent := `
scanner:
  interval_seconds: 999
  timeout_seconds: 999
`
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	// Try to import - should fail because not first run
	err = ImportYAMLConfig(yamlPath, db)
	if err == nil {
		t.Fatal("Expected ImportYAMLConfig to fail when not first run")
	}

	// Verify settings were NOT changed
	settings, err := db.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Scanner.IntervalSeconds == 999 {
		t.Error("Settings should not have been updated when not first run")
	}

	if settings.Scanner.IntervalSeconds != 300 {
		t.Errorf("Expected original interval_seconds=300, got %d", settings.Scanner.IntervalSeconds)
	}
}

func TestImportYAMLConfig_MultipleEndpoints(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test YAML with multiple endpoints
	yamlContent := `
telemetry:
  interval_hours: 72
  endpoints:
    - name: endpoint1
      url: https://endpoint1.com
      enabled: true
      api_key: key1
    - name: endpoint2
      url: https://endpoint2.com
      enabled: false
      api_key: key2
    - name: endpoint3
      url: https://endpoint3.com
      enabled: true
`
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	// Import
	err = ImportYAMLConfig(yamlPath, db)
	if err != nil {
		t.Fatalf("ImportYAMLConfig failed: %v", err)
	}

	// Verify all endpoints were imported
	endpoints, err := db.GetTelemetryEndpoints()
	if err != nil {
		t.Fatalf("Failed to load telemetry endpoints: %v", err)
	}

	if len(endpoints) != 3 {
		t.Fatalf("Expected 3 endpoints, got %d", len(endpoints))
	}

	// Check each endpoint
	endpointMap := make(map[string]models.TelemetryEndpoint)
	for _, ep := range endpoints {
		endpointMap[ep.Name] = ep
	}

	ep1, ok := endpointMap["endpoint1"]
	if !ok {
		t.Fatal("endpoint1 not found")
	}
	if !ep1.Enabled {
		t.Error("endpoint1 should be enabled")
	}
	if ep1.APIKey != "key1" {
		t.Errorf("endpoint1 API key should be 'key1', got '%s'", ep1.APIKey)
	}

	ep2, ok := endpointMap["endpoint2"]
	if !ok {
		t.Fatal("endpoint2 not found")
	}
	if ep2.Enabled {
		t.Error("endpoint2 should be disabled")
	}

	ep3, ok := endpointMap["endpoint3"]
	if !ok {
		t.Fatal("endpoint3 not found")
	}
	if !ep3.Enabled {
		t.Error("endpoint3 should be enabled")
	}
}

func TestImportYAMLConfig_HostsNotOverwritten(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Add a host manually before import
	existingHost := models.Host{
		Name:        "existing-host",
		Address:     "tcp://existing:2375",
		Description: "Existing host",
		Enabled:     true,
	}
	if _, err := db.AddHost(existingHost); err != nil {
		t.Fatalf("Failed to add existing host: %v", err)
	}

	// Now save settings to trigger "not first run" for hosts
	defaults := storage.GetDefaultSettings()
	if err := db.SaveSystemSettings(defaults); err != nil {
		t.Fatalf("Failed to save settings: %v", err)
	}

	// Create YAML with different hosts
	yamlContent := `
hosts:
  - name: yaml-host
    address: unix:///var/run/docker.sock
    description: Host from YAML
`
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	// This should fail because not first run
	err = ImportYAMLConfig(yamlPath, db)
	if err == nil {
		t.Fatal("Expected import to fail when settings already exist")
	}

	// Verify only the existing host is present
	hosts, err := db.GetHosts()
	if err != nil {
		t.Fatalf("Failed to load hosts: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(hosts))
	}

	if hosts[0].Name != "existing-host" {
		t.Errorf("Expected existing host to remain, got '%s'", hosts[0].Name)
	}
}

func TestIsFirstRun(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Should be first run initially
	if !db.IsFirstRun() {
		t.Error("Expected IsFirstRun to be true for new database")
	}

	// Save settings
	defaults := storage.GetDefaultSettings()
	if err := db.SaveSystemSettings(defaults); err != nil {
		t.Fatalf("Failed to save settings: %v", err)
	}

	// Should no longer be first run
	if db.IsFirstRun() {
		t.Error("Expected IsFirstRun to be false after saving settings")
	}
}
