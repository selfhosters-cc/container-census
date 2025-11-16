package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/container-census/container-census/internal/migration"
	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
	"gopkg.in/yaml.v3"
)

// handleGetSettings returns all system settings from the database
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.LoadSystemSettings()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load settings: %v", err), http.StatusInternalServerError)
		return
	}

	// Load telemetry endpoints
	endpoints, err := s.db.GetTelemetryEndpoints()
	if err != nil {
		log.Printf("Warning: Failed to load telemetry endpoints: %v", err)
		endpoints = []models.TelemetryEndpoint{}
	}

	// Build response combining settings and endpoints
	response := map[string]interface{}{
		"scanner": settings.Scanner,
		"telemetry": map[string]interface{}{
			"interval_hours": settings.Telemetry.IntervalHours,
			"endpoints":      endpoints,
		},
		"notification": settings.Notification,
		"ui":           settings.UI,
		"updated_at":   settings.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleUpdateSettings updates system settings in the database and triggers hot-reload
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings models.SystemSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate settings
	if err := settings.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Invalid settings: %v", err), http.StatusBadRequest)
		return
	}

	// Save to database
	if err := s.db.SaveSystemSettings(&settings); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	log.Println("Settings updated via API, triggering hot-reload...")

	// Trigger hot-reload if callback is set
	if s.reloadSettingsFunc != nil {
		if err := s.reloadSettingsFunc(); err != nil {
			log.Printf("Warning: Hot-reload failed: %v", err)
			// Don't fail the request - settings are saved, just restart didn't work
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Settings updated and reloaded successfully",
	})
}

// handleExportSettings exports current settings to YAML format
func (s *Server) handleExportSettings(w http.ResponseWriter, r *http.Request) {
	yamlData, err := migration.ExportToYAML(s.db)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to export settings: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", "attachment; filename=container-census-config.yaml")
	w.Write(yamlData)
}

// handleImportSettings imports settings from YAML format
func (s *Server) handleImportSettings(w http.ResponseWriter, r *http.Request) {
	// Read uploaded YAML file
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	yamlData := make([]byte, 0)
	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			yamlData = append(yamlData, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	// Parse YAML to Config struct
	var cfg models.Config
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
		http.Error(w, fmt.Sprintf("Invalid YAML: %v", err), http.StatusBadRequest)
		return
	}

	// Convert Config to SystemSettings
	settings := &models.SystemSettings{
		Scanner: models.ScannerSettings{
			IntervalSeconds: cfg.Scanner.IntervalSeconds,
			TimeoutSeconds:  cfg.Scanner.TimeoutSeconds,
		},
		Telemetry: models.TelemetrySettings{
			IntervalHours: cfg.Telemetry.IntervalHours,
		},
		Notification: models.NotificationSettings{
			RateLimitMax:           100,  // Use existing defaults
			RateLimitBatchInterval: 600,
			ThresholdDuration:      120,
			CooldownPeriod:         300,
		},
	}

	// Validate settings
	if err := settings.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Invalid settings in YAML: %v", err), http.StatusBadRequest)
		return
	}

	// Save to database
	if err := s.db.SaveSystemSettings(settings); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	// Import telemetry endpoints if present
	for _, ep := range cfg.Telemetry.Endpoints {
		if err := s.db.SaveTelemetryEndpoint(&ep); err != nil {
			log.Printf("Warning: Failed to import telemetry endpoint %s: %v", ep.Name, err)
		}
	}

	log.Println("Settings imported from YAML, triggering hot-reload...")

	// Trigger hot-reload
	if s.reloadSettingsFunc != nil {
		if err := s.reloadSettingsFunc(); err != nil {
			log.Printf("Warning: Hot-reload failed: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Settings imported and reloaded successfully",
	})
}

// handleGetMigrationStatus checks if the config migration has been completed
func (s *Server) handleGetMigrationStatus(w http.ResponseWriter, r *http.Request) {
	migrated, err := s.db.GetPreference("config_migrated")
	if err != nil {
		// No preference set means not migrated yet
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"migrated": false,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"migrated": migrated == "true",
	})
}

// handleAcknowledgeMigration marks the migration notification as acknowledged
func (s *Server) handleAcknowledgeMigration(w http.ResponseWriter, r *http.Request) {
	if err := s.db.SetPreference("migration_acknowledged", "true"); err != nil {
		http.Error(w, fmt.Sprintf("Failed to acknowledge migration: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// ======= DANGER ZONE ENDPOINTS =======

// handleResetSettings deletes all system settings from database (will trigger auto-import on next load)
func (s *Server) handleResetSettings(w http.ResponseWriter, r *http.Request) {
	log.Println("⚠️ DANGER ZONE: Resetting all settings to defaults...")

	// Delete the config_migrated preference to trigger reimport
	if err := s.db.SetPreference("config_migrated", "false"); err != nil {
		log.Printf("Warning: Failed to reset migration flag: %v", err)
	}

	// Delete all system settings by saving defaults
	defaults := storage.GetDefaultSettings()
	if err := s.db.SaveSystemSettings(defaults); err != nil {
		http.Error(w, fmt.Sprintf("Failed to reset settings: %v", err), http.StatusInternalServerError)
		return
	}

	// Delete all telemetry endpoints
	endpoints, _ := s.db.GetTelemetryEndpoints()
	for _, ep := range endpoints {
		if err := s.db.DeleteTelemetryEndpoint(ep.Name); err != nil {
			log.Printf("Warning: Failed to delete endpoint %s: %v", ep.Name, err)
		}
	}

	log.Println("✅ Settings reset to defaults")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "All settings reset to defaults. Refresh the page to trigger auto-import from config.yaml if available.",
	})
}

// handleClearContainerHistory deletes all historical container data (keeps current snapshot)
func (s *Server) handleClearContainerHistory(w http.ResponseWriter, r *http.Request) {
	log.Println("⚠️ DANGER ZONE: Clearing container history...")

	deleted, err := s.db.ClearOldContainerHistory(0) // 0 = delete all history
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to clear history: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Cleared %d historical container records", deleted)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Deleted %d historical container records", deleted),
		"deleted": deleted,
	})
}

// handleClearVulnerabilities deletes all vulnerability scan data
func (s *Server) handleClearVulnerabilities(w http.ResponseWriter, r *http.Request) {
	log.Println("⚠️ DANGER ZONE: Clearing all vulnerability data...")

	// Use zero retention days to delete everything
	if err := s.db.CleanupOldVulnerabilityData(0, 0); err != nil {
		http.Error(w, fmt.Sprintf("Failed to clear vulnerabilities: %v", err), http.StatusInternalServerError)
		return
	}

	log.Println("✅ All vulnerability data cleared")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "All vulnerability scans and CVE data deleted",
	})
}

// handleClearActivityLog deletes all lifecycle events and activity history
func (s *Server) handleClearActivityLog(w http.ResponseWriter, r *http.Request) {
	log.Println("⚠️ DANGER ZONE: Clearing activity log...")

	// Delete all lifecycle events
	if err := s.db.ClearAllLifecycleEvents(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to clear activity log: %v", err), http.StatusInternalServerError)
		return
	}

	log.Println("✅ Activity log cleared")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "All lifecycle events and activity history deleted",
	})
}

// handleNuclearReset deletes EVERYTHING from the database
func (s *Server) handleNuclearReset(w http.ResponseWriter, r *http.Request) {
	log.Println("💀 DANGER ZONE: NUCLEAR RESET - Deleting ALL data...")

	stats := map[string]interface{}{}

	// 1. Clear container history
	deleted, err := s.db.ClearOldContainerHistory(0)
	if err != nil {
		log.Printf("Warning: Failed to clear container history: %v", err)
	} else {
		stats["containers_deleted"] = deleted
	}

	// 2. Clear vulnerabilities
	if err := s.db.CleanupOldVulnerabilityData(0, 0); err != nil {
		log.Printf("Warning: Failed to clear vulnerabilities: %v", err)
	}

	// 3. Clear activity log
	if err := s.db.ClearAllLifecycleEvents(); err != nil {
		log.Printf("Warning: Failed to clear activity log: %v", err)
	}

	// 4. Delete all hosts
	hosts, _ := s.db.GetHosts()
	for _, host := range hosts {
		if err := s.db.DeleteHost(host.ID); err != nil {
			log.Printf("Warning: Failed to delete host %s: %v", host.Name, err)
		}
	}
	stats["hosts_deleted"] = len(hosts)

	// 5. Delete all telemetry endpoints
	endpoints, _ := s.db.GetTelemetryEndpoints()
	for _, ep := range endpoints {
		if err := s.db.DeleteTelemetryEndpoint(ep.Name); err != nil {
			log.Printf("Warning: Failed to delete endpoint %s: %v", ep.Name, err)
		}
	}
	stats["endpoints_deleted"] = len(endpoints)

	// 6. Clear all notifications
	if err := s.db.ClearAllNotifications(); err != nil {
		log.Printf("Warning: Failed to clear notifications: %v", err)
	}

	// 7. Reset settings to defaults
	defaults := storage.GetDefaultSettings()
	if err := s.db.SaveSystemSettings(defaults); err != nil {
		log.Printf("Warning: Failed to reset settings: %v", err)
	}

	// 8. Reset migration flag
	if err := s.db.SetPreference("config_migrated", "false"); err != nil {
		log.Printf("Warning: Failed to reset migration flag: %v", err)
	}

	log.Println("💀 NUCLEAR RESET COMPLETE - All data deleted")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "All data deleted. Database reset to fresh installation state.",
		"stats":   stats,
	})
}
