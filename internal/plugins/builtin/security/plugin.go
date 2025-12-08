package security

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/container-census/container-census/internal/plugins"
	"github.com/container-census/container-census/internal/scanner"
	"github.com/container-census/container-census/internal/storage"
	"github.com/container-census/container-census/internal/vulnerability"
)

//go:embed frontend/bundle.js
var bundleJS []byte

// SecurityPlugin implements vulnerability scanning as a plugin
type SecurityPlugin struct {
	deps          plugins.PluginDependencies
	db            *storage.DB // Raw database access for vulnerability methods
	scanner       *scanner.Scanner
	vulnScanner   *vulnerability.Scanner
	vulnScheduler *vulnerability.Scheduler
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
}

// NewSecurityPlugin creates a new security plugin instance
func NewSecurityPlugin() *SecurityPlugin {
	return &SecurityPlugin{}
}

// Info returns plugin metadata
func (p *SecurityPlugin) Info() plugins.PluginInfo {
	return plugins.PluginInfo{
		ID:          "security",
		Name:        "Security Scanner",
		Version:     "1.0.0",
		Description: "Vulnerability scanning with Trivy integration",
		Author:      "Container Census Team",
		Capabilities: []string{
			"vulnerability_scanning",
			"ui_tab",
			"ui_badge",
			"settings",
		},
		BuiltIn: true,
	}
}

// Init initializes the plugin
func (p *SecurityPlugin) Init(ctx context.Context, deps plugins.PluginDependencies) error {
	p.deps = deps
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Get raw database access
	// The deps.DB is a scopedPluginDB which wraps *storage.DB
	// We need to access the underlying DB for vulnerability methods
	p.db = p.getRawDB(deps.DB)
	if p.db == nil {
		return fmt.Errorf("failed to get raw database access")
	}

	// Get scanner from dependencies (need to add this to dependencies)
	// For now, we'll initialize in Start() when we have access to the scanner
	deps.Logger.Info("Security plugin initialized")

	return nil
}

// getRawDB extracts the raw storage.DB from the scoped PluginDB
func (p *SecurityPlugin) getRawDB(pluginDB plugins.PluginDB) *storage.DB {
	// Type assert to access the underlying DB
	// The scoped plugin DB now has a GetRawDB() method for trusted built-in plugins
	type dbGetter interface {
		GetRawDB() *storage.DB
	}

	if getter, ok := pluginDB.(dbGetter); ok {
		return getter.GetRawDB()
	}

	log.Printf("Error: Could not access raw database - plugin DB does not implement GetRawDB()")
	return nil
}

// Start starts the plugin background tasks
func (p *SecurityPlugin) Start(ctx context.Context) error {
	p.deps.Logger.Info("Starting Security plugin...")

	// Load vulnerability configuration from database
	vulnConfig, err := p.db.LoadVulnerabilitySettings()
	if err != nil {
		p.deps.Logger.Error(fmt.Sprintf("Failed to load vulnerability settings: %v", err))
		// Use default config
		vulnConfig = vulnerability.DefaultConfig()
	}

	if !vulnConfig.GetEnabled() {
		p.deps.Logger.Info("Vulnerability scanning is disabled")
		return nil
	}

	// Initialize vulnerability scanner
	p.vulnScanner = vulnerability.NewScanner(vulnConfig, p.db)

	// Initialize vulnerability scheduler
	p.vulnScheduler = vulnerability.NewScheduler(p.vulnScanner, vulnConfig)

	// Start scheduler
	p.vulnScheduler.Start()

	// Start background jobs
	go p.runDailyTrivyUpdate(p.ctx, vulnConfig)
	go p.runDailyCleanup(p.ctx, vulnConfig)

	p.deps.Logger.Info("Security plugin started successfully")
	return nil
}

// Stop stops the plugin
func (p *SecurityPlugin) Stop(ctx context.Context) error {
	p.deps.Logger.Info("Stopping Security plugin...")

	if p.cancel != nil {
		p.cancel()
	}

	p.deps.Logger.Info("Security plugin stopped")
	return nil
}

// Routes returns the plugin's API routes
func (p *SecurityPlugin) Routes() []plugins.Route {
	return []plugins.Route{
		// Bundle serving
		{Path: "/bundle.js", Method: "GET", Handler: p.serveBundleJS},

		// API endpoints
		{Path: "/summary", Method: "GET", Handler: p.handleGetSummary},
		{Path: "/scans", Method: "GET", Handler: p.handleGetScans},
		{Path: "/image/{imageId}", Method: "GET", Handler: p.handleGetImage},
		{Path: "/container/{hostId}/{containerId}", Method: "GET", Handler: p.handleGetContainer},
		{Path: "/scan/{imageId}", Method: "POST", Handler: p.handleTriggerScan},
		{Path: "/scan-all", Method: "POST", Handler: p.handleScanAll},
		{Path: "/queue", Method: "GET", Handler: p.handleGetQueue},
		{Path: "/update-db", Method: "POST", Handler: p.handleUpdateDB},
		{Path: "/settings", Method: "GET", Handler: p.handleGetSettings},
		{Path: "/settings", Method: "PUT", Handler: p.handleUpdateSettings},
		{Path: "/clear", Method: "POST", Handler: p.handleClear},
	}
}

// Tab returns the tab definition
func (p *SecurityPlugin) Tab() *plugins.TabDefinition {
	contentHTML := `
		<div class="security-section-modern">
			<div class="security-header-modern">
				<div class="security-title-group">
					<h2>🛡️ Vulnerability Scanner</h2>
					<p class="security-subtitle">Monitor and track security vulnerabilities across all container images</p>
				</div>
				<div class="security-actions">
					<button onclick="window.scanAllImages()" class="btn btn-primary">🔄 Scan All Images</button>
					<button onclick="window.updateTrivyDB()" class="btn btn-secondary">📥 Update Database</button>
					<button onclick="window.showSecuritySettings()" class="btn btn-secondary">⚙️ Settings</button>
					<button onclick="window.exportVulnerabilityData()" class="btn btn-secondary">📊 Export</button>
				</div>
			</div>

			<div id="vulnerabilitySummary"></div>

			<div class="security-table-card">
				<div class="security-table-header-modern">
					<div class="table-title-group">
						<h3>Vulnerability Scans</h3>
						<span class="scan-count" id="scanCountBadge">0 scans</span>
					</div>
					<div class="security-filters-modern">
						<input type="text" id="securitySearch" class="search-input" placeholder="🔍 Search images..." oninput="window.filterSecurityScans()">
					</div>
				</div>
				<div class="table-container">
					<table class="vulnerability-table" id="securityScansTable">
						<thead>
							<tr>
								<th>Image Name</th>
								<th>Status</th>
								<th>Total Vulnerabilities</th>
								<th>Severity Breakdown</th>
								<th>Scanned At</th>
							</tr>
						</thead>
						<tbody>
							<tr>
								<td colspan="5" class="loading">Loading vulnerability scans...</td>
							</tr>
						</tbody>
					</table>
				</div>
			</div>
		</div>
	`

	return &plugins.TabDefinition{
		ID:          "security",
		Label:       "Security",
		Icon:        "🛡️",
		Order:       12, // After containers (10), before integrations (14)
		ContentHTML: contentHTML,
		ScriptURL:   "/bundle.js", // Relative to /api/p/security/
		InitFunc:    "initSecurityPlugin",
	}
}

// Badges returns badge providers
func (p *SecurityPlugin) Badges() []plugins.BadgeProvider {
	// TODO: Implement vulnerability badges
	return nil
}

// ContainerEnricher returns nil (not needed)
func (p *SecurityPlugin) ContainerEnricher() plugins.ContainerEnricher {
	return nil
}

// Settings returns settings definition
func (p *SecurityPlugin) Settings() *plugins.SettingsDefinition {
	// TODO: Implement settings schema
	return nil
}

// NotificationChannelFactory returns nil (not needed)
func (p *SecurityPlugin) NotificationChannelFactory() plugins.ChannelFactory {
	return nil
}

// serveBundleJS serves the embedded JavaScript bundle
func (p *SecurityPlugin) serveBundleJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(bundleJS)
}

// runDailyTrivyUpdate runs daily Trivy database updates
func (p *SecurityPlugin) runDailyTrivyUpdate(ctx context.Context, config *vulnerability.Config) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Calculate time until 2 AM
	now := time.Now()
	next2AM := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
	if now.After(next2AM) {
		next2AM = next2AM.Add(24 * time.Hour)
	}
	initialDelay := time.Until(next2AM)

	p.deps.Logger.Info(fmt.Sprintf("Trivy DB update scheduled for %s (in %v)", next2AM.Format("2006-01-02 15:04:05"), initialDelay))

	// Wait for initial delay
	select {
	case <-time.After(initialDelay):
		p.updateTrivyDB(ctx)
	case <-ctx.Done():
		return
	}

	// Then run every 24 hours
	for {
		select {
		case <-ticker.C:
			p.updateTrivyDB(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// updateTrivyDB updates the Trivy vulnerability database
func (p *SecurityPlugin) updateTrivyDB(ctx context.Context) {
	p.deps.Logger.Info("Running daily Trivy database update...")
	if p.vulnScanner == nil {
		p.deps.Logger.Error("Vulnerability scanner not initialized")
		return
	}

	updateCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := p.vulnScanner.UpdateTrivyDB(updateCtx); err != nil {
		p.deps.Logger.Error(fmt.Sprintf("Failed to update Trivy database: %v", err))
	} else {
		p.deps.Logger.Info("Trivy database updated successfully")
	}
}

// runDailyCleanup runs daily cleanup of old vulnerability data
func (p *SecurityPlugin) runDailyCleanup(ctx context.Context, config *vulnerability.Config) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Calculate time until 3 AM
	now := time.Now()
	next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if now.After(next3AM) {
		next3AM = next3AM.Add(24 * time.Hour)
	}
	initialDelay := time.Until(next3AM)

	p.deps.Logger.Info(fmt.Sprintf("Vulnerability cleanup scheduled for %s (in %v)", next3AM.Format("2006-01-02 15:04:05"), initialDelay))

	// Wait for initial delay
	select {
	case <-time.After(initialDelay):
		p.cleanupOldScans(ctx, config)
	case <-ctx.Done():
		return
	}

	// Then run every 24 hours
	for {
		select {
		case <-ticker.C:
			p.cleanupOldScans(ctx, config)
		case <-ctx.Done():
			return
		}
	}
}

// cleanupOldScans cleans up old vulnerability scan data
func (p *SecurityPlugin) cleanupOldScans(ctx context.Context, config *vulnerability.Config) {
	p.deps.Logger.Info("Running daily vulnerability cleanup...")
	if p.db == nil {
		p.deps.Logger.Error("Database not initialized")
		return
	}

	// Use the cleanup method from storage.DB
	retentionDays := config.GetRetentionDays()
	detailedRetentionDays := config.GetDetailedRetentionDays()

	if err := p.db.CleanupOldVulnerabilityData(retentionDays, detailedRetentionDays); err != nil {
		p.deps.Logger.Error(fmt.Sprintf("Vulnerability cleanup failed: %v", err))
	} else {
		p.deps.Logger.Info("Vulnerability cleanup completed successfully")
	}
}
