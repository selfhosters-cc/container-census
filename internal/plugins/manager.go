package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/plugins/external"
	pb "github.com/container-census/container-census/internal/plugins/proto"
	"github.com/container-census/container-census/internal/storage"
	"github.com/gorilla/mux"
)

// Manager manages plugin lifecycle and provides access to plugin features
type Manager struct {
	mu               sync.RWMutex
	plugins          map[string]Plugin
	pluginOrder      []string // Order of plugin registration
	builtInFactories map[string]PluginFactory
	db               *storage.DB
	containers       ContainerProvider
	hosts            HostProvider
	eventBus         *EventBusImpl
	router           *mux.Router
	started          bool

	// External plugin support
	installer       *external.PluginInstaller
	supervisor      *external.ExternalPluginSupervisor
	censusAPIServer *external.CensusAPIServer
	pluginsDir      string
}

// PluginFactory creates a new plugin instance
type PluginFactory func() Plugin

// NewManager creates a new plugin manager
func NewManager(db *storage.DB, containers ContainerProvider, hosts HostProvider) *Manager {
	// Default to /app/data/plugins, but use ./data/plugins for local development
	pluginsDir := "/app/data/plugins"
	if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
		pluginsDir = dataDir + "/plugins"
	} else if _, err := os.Stat("/app/data"); os.IsNotExist(err) {
		// Running locally, not in Docker container
		pluginsDir = "./data/plugins"
	}

	return &Manager{
		plugins:          make(map[string]Plugin),
		pluginOrder:      make([]string, 0),
		builtInFactories: make(map[string]PluginFactory),
		db:               db,
		containers:       containers,
		hosts:            hosts,
		eventBus:         NewEventBus(),

		// External plugin infrastructure
		installer:       external.NewPluginInstaller(db, pluginsDir),
		supervisor:      external.NewExternalPluginSupervisor(db, "localhost:50052", 50100),
		censusAPIServer: external.NewCensusAPIServer(db),
		pluginsDir:      pluginsDir,
	}
}

// SetRouter sets the router for mounting plugin routes
func (m *Manager) SetRouter(router *mux.Router) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.router = router
}

// GetCensusAPIServer returns the Census API server for plugin callbacks
func (m *Manager) GetCensusAPIServer() *external.CensusAPIServer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.censusAPIServer
}

// RegisterBuiltIn registers a built-in plugin factory
func (m *Manager) RegisterBuiltIn(id string, factory PluginFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.builtInFactories[id] = factory
	log.Printf("Registered built-in plugin factory: %s", id)
}

// LoadBuiltInPlugins loads all registered built-in plugins
func (m *Manager) LoadBuiltInPlugins(ctx context.Context) error {
	m.mu.Lock()
	factories := make(map[string]PluginFactory)
	for id, f := range m.builtInFactories {
		factories[id] = f
	}
	m.mu.Unlock()

	for id, factory := range factories {
		// Check if plugin is disabled in database
		record, err := m.db.GetPlugin(id)
		if err == nil && record != nil && !record.Enabled {
			log.Printf("Skipping disabled plugin: %s", id)
			continue
		}

		plugin := factory()
		if err := m.loadPlugin(ctx, plugin); err != nil {
			log.Printf("Failed to load built-in plugin %s: %v", id, err)
			continue
		}
	}

	return nil
}

// LoadExternalPlugins loads and starts all enabled external plugins from database
func (m *Manager) LoadExternalPlugins(ctx context.Context) error {
	// Get all plugin records from database
	records, err := m.db.GetAllPlugins()
	if err != nil {
		return fmt.Errorf("failed to get plugins from database: %w", err)
	}

	// Filter for enabled external plugins
	for _, record := range records {
		// Skip built-in plugins (they're loaded via LoadBuiltInPlugins)
		if record.SourceType == "built_in" {
			continue
		}

		// Skip disabled plugins
		if !record.Enabled {
			log.Printf("Skipping disabled external plugin: %s", record.ID)
			continue
		}

		// Start the external plugin
		log.Printf("Loading external plugin: %s v%s", record.Name, record.Version)
		if err := m.StartExternalPlugin(ctx, record.ID); err != nil {
			log.Printf("Failed to start external plugin %s: %v", record.ID, err)
			continue
		}

		// Mount routes for the plugin with retry (gRPC client needs time to connect)
		mounted := false
		for attempt := 1; attempt <= 5; attempt++ {
			if attempt > 1 {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			}
			if err := m.MountPluginRoutes(record.ID); err != nil {
				if attempt < 5 {
					log.Printf("[PluginManager] Attempt %d/5: Failed to mount routes for %s, retrying...", attempt, record.ID)
					continue
				}
				log.Printf("Failed to mount routes for plugin %s after 5 attempts: %v", record.ID, err)
			} else {
				mounted = true
				break
			}
		}

		if !mounted {
			log.Printf("Warning: Plugin %s started but routes not mounted", record.ID)
		} else {
			// Fetch and save tab configuration after successful route mounting
			if err := m.FetchAndSaveTabConfig(ctx, record.ID); err != nil {
				log.Printf("Warning: Failed to fetch tab config for %s: %v", record.ID, err)
				// Continue anyway - tab can be fetched later if needed
			}
		}

		log.Printf("Loaded external plugin: %s v%s", record.Name, record.Version)
	}

	return nil
}

// loadPlugin initializes and registers a plugin
func (m *Manager) loadPlugin(ctx context.Context, plugin Plugin) error {
	info := plugin.Info()

	// Create scoped dependencies
	deps := PluginDependencies{
		DB:         &scopedPluginDB{db: m.db, pluginID: info.ID},
		Containers: m.containers,
		Hosts:      m.hosts,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Logger:     &DefaultPluginLogger{Prefix: info.ID},
		EventBus:   m.eventBus,
	}

	// Initialize plugin
	if err := plugin.Init(ctx, deps); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", info.ID, err)
	}

	// Save plugin record
	record := &storage.PluginRecord{
		ID:          info.ID,
		Name:        info.Name,
		Version:     info.Version,
		SourceType:  "built_in",
		Enabled:     true,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	if !info.BuiltIn {
		record.SourceType = "github"
	}
	if err := m.db.SavePlugin(record); err != nil {
		log.Printf("Warning: failed to save plugin record for %s: %v", info.ID, err)
	}

	// Register plugin
	m.mu.Lock()
	m.plugins[info.ID] = plugin
	m.pluginOrder = append(m.pluginOrder, info.ID)
	m.mu.Unlock()

	// Mount routes if router is available
	if m.router != nil {
		m.mountPluginRoutes(info.ID, plugin)
	}

	log.Printf("Loaded plugin: %s v%s", info.Name, info.Version)
	return nil
}

// mountPluginRoutes mounts a plugin's API routes
func (m *Manager) mountPluginRoutes(pluginID string, plugin Plugin) {
	routes := plugin.Routes()
	if len(routes) == 0 {
		return
	}

	for _, route := range routes {
		// Router is already prefixed with /api, so we add /p/{id}{path}
		// Using /p/ instead of /plugins/ to avoid conflict with /plugins/{id} management routes
		path := fmt.Sprintf("/p/%s%s", pluginID, route.Path)
		m.router.HandleFunc(path, route.Handler).Methods(route.Method)
		log.Printf("Mounted plugin route: %s /api%s", route.Method, path)
	}
}

// Start starts all loaded plugins
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	plugins := make([]Plugin, 0, len(m.plugins))
	for _, id := range m.pluginOrder {
		plugins = append(plugins, m.plugins[id])
	}
	m.mu.Unlock()

	for _, plugin := range plugins {
		info := plugin.Info()
		if err := plugin.Start(ctx); err != nil {
			log.Printf("Failed to start plugin %s: %v", info.ID, err)
			continue
		}
		log.Printf("Started plugin: %s", info.Name)
	}

	m.mu.Lock()
	m.started = true
	m.mu.Unlock()

	return nil
}

// Stop stops all plugins
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	plugins := make([]Plugin, 0, len(m.plugins))
	// Stop in reverse order
	for i := len(m.pluginOrder) - 1; i >= 0; i-- {
		plugins = append(plugins, m.plugins[m.pluginOrder[i]])
	}
	m.mu.Unlock()

	for _, plugin := range plugins {
		info := plugin.Info()
		if err := plugin.Stop(ctx); err != nil {
			log.Printf("Failed to stop plugin %s: %v", info.ID, err)
			continue
		}
		log.Printf("Stopped plugin: %s", info.Name)
	}

	m.mu.Lock()
	m.started = false
	m.mu.Unlock()

	return nil
}

// GetPlugin returns a plugin by ID
func (m *Manager) GetPlugin(id string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	plugin, ok := m.plugins[id]
	return plugin, ok
}

// GetAllPlugins returns all loaded plugins
func (m *Manager) GetAllPlugins() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	plugins := make([]Plugin, 0, len(m.plugins))
	for _, id := range m.pluginOrder {
		plugins = append(plugins, m.plugins[id])
	}
	return plugins
}

// GetAllPluginInfo returns info for all plugins (including disabled)
func (m *Manager) GetAllPluginInfo() ([]PluginInfo, error) {
	// Get registered built-in plugins
	m.mu.RLock()
	loadedPlugins := make(map[string]Plugin)
	for id, p := range m.plugins {
		loadedPlugins[id] = p
	}
	builtInIDs := make([]string, 0, len(m.builtInFactories))
	for id := range m.builtInFactories {
		builtInIDs = append(builtInIDs, id)
	}
	m.mu.RUnlock()

	// Get database records
	records, err := m.db.GetAllPlugins()
	if err != nil {
		return nil, err
	}
	recordMap := make(map[string]*storage.PluginRecord)
	for _, r := range records {
		recordMap[r.ID] = r
	}

	var result []PluginInfo

	// Add loaded plugins
	for _, plugin := range loadedPlugins {
		info := plugin.Info()
		result = append(result, info)
	}

	// Add disabled built-in plugins
	for _, id := range builtInIDs {
		if _, loaded := loadedPlugins[id]; !loaded {
			if record, exists := recordMap[id]; exists && !record.Enabled {
				// Create factory to get info
				factory := m.builtInFactories[id]
				plugin := factory()
				info := plugin.Info()
				result = append(result, info)
			}
		}
	}

	// Add external plugins from database
	for _, record := range records {
		// Skip if already added (loaded or disabled built-in)
		if _, loaded := loadedPlugins[record.ID]; loaded {
			continue
		}
		if _, isBuiltIn := m.builtInFactories[record.ID]; isBuiltIn {
			continue
		}

		// Create PluginInfo from database record for external plugins
		info := PluginInfo{
			ID:           record.ID,
			Name:         record.Name,
			Version:      record.Version,
			Description:  "", // External plugins don't have description in DB yet
			Author:       "",
			Homepage:     record.SourceURL,
			Capabilities: []string{"ui_tab"}, // External plugins with tabs
			BuiltIn:      false,
		}
		result = append(result, info)
	}

	return result, nil
}

// GetAllTabs returns tab definitions from all plugins
func (m *Manager) GetAllTabs() []TabDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tabs := make([]TabDefinition, 0)

	// Add tabs from loaded (built-in) plugins
	log.Printf("[DEBUG] GetAllTabs: Processing %d built-in plugins", len(m.pluginOrder))
	for _, id := range m.pluginOrder {
		plugin := m.plugins[id]
		if tab := plugin.Tab(); tab != nil {
			log.Printf("[DEBUG] GetAllTabs: Added built-in tab: %s (%s)", tab.ID, tab.Label)
			tabs = append(tabs, *tab)
		}
	}

	// Add tabs from external plugins stored in database
	records, err := m.db.GetAllPlugins()
	log.Printf("[DEBUG] GetAllTabs: Retrieved %d plugin records from database, err: %v", len(records), err)
	if err == nil {
		for _, record := range records {
			log.Printf("[DEBUG] GetAllTabs: Processing plugin %s, enabled=%v, tab_config=%q", record.ID, record.Enabled, record.TabConfig)

			// Skip disabled plugins
			if !record.Enabled {
				log.Printf("[DEBUG] GetAllTabs: Skipping disabled plugin %s", record.ID)
				continue
			}

			// Skip built-in plugins (already handled above)
			if _, isBuiltIn := m.builtInFactories[record.ID]; isBuiltIn {
				log.Printf("[DEBUG] GetAllTabs: Skipping built-in plugin %s", record.ID)
				continue
			}

			// Parse tab_config from database (it's stored as JSON string)
			if record.TabConfig != "" {
				// Unmarshal JSON string to map
				var tabConfig map[string]string
				if err := json.Unmarshal([]byte(record.TabConfig), &tabConfig); err != nil {
					log.Printf("[DEBUG] GetAllTabs: Failed to unmarshal tab_config for %s: %v", record.ID, err)
					continue
				}

				// Convert map to TabDefinition
				tab := TabDefinition{
					ID:        tabConfig["id"],
					Label:     tabConfig["label"],
					Icon:      tabConfig["icon"],
					ScriptURL: tabConfig["script_url"],
					InitFunc:  tabConfig["init_func"],
				}

				// Parse order from string
				if orderStr := tabConfig["order"]; orderStr != "" {
					if order, err := strconv.Atoi(orderStr); err == nil {
						tab.Order = order
					}
				}

				log.Printf("[DEBUG] GetAllTabs: Successfully parsed tab for %s: %+v", record.ID, tab)
				tabs = append(tabs, tab)
			} else {
				log.Printf("[DEBUG] GetAllTabs: Empty tab_config for plugin %s", record.ID)
			}
		}
	}

	// Sort by order
	sort.Slice(tabs, func(i, j int) bool {
		return tabs[i].Order < tabs[j].Order
	})

	log.Printf("[DEBUG] GetAllTabs: Returning %d total tabs", len(tabs))
	return tabs
}

// GetBadgesForContainer returns all badges for a container from all plugins
func (m *Manager) GetBadgesForContainer(ctx context.Context, container models.Container) []Badge {
	m.mu.RLock()
	plugins := make([]Plugin, 0, len(m.plugins))
	for _, id := range m.pluginOrder {
		plugins = append(plugins, m.plugins[id])
	}
	m.mu.RUnlock()

	var badges []Badge
	for _, plugin := range plugins {
		providers := plugin.Badges()
		for _, provider := range providers {
			badge, err := provider.GetBadge(ctx, container)
			if err != nil {
				log.Printf("Error getting badge from plugin: %v", err)
				continue
			}
			if badge != nil {
				badge.PluginID = plugin.Info().ID
				badges = append(badges, *badge)
			}
		}
	}

	// Sort by priority (higher first)
	sort.Slice(badges, func(i, j int) bool {
		return badges[i].Priority > badges[j].Priority
	})

	return badges
}

// EnrichContainer enriches a container with data from all plugins
func (m *Manager) EnrichContainer(ctx context.Context, container *models.Container) {
	m.mu.RLock()
	plugins := make([]Plugin, 0, len(m.plugins))
	for _, id := range m.pluginOrder {
		plugins = append(plugins, m.plugins[id])
	}
	m.mu.RUnlock()

	if container.PluginData == nil {
		container.PluginData = make(map[string]interface{})
	}

	for _, plugin := range plugins {
		enricher := plugin.ContainerEnricher()
		if enricher == nil {
			continue
		}
		if err := enricher.Enrich(ctx, container); err != nil {
			log.Printf("Error enriching container from plugin %s: %v", plugin.Info().ID, err)
		}
	}
}

// EnablePlugin enables a plugin
func (m *Manager) EnablePlugin(ctx context.Context, id string) error {
	if err := m.db.SetPluginEnabled(id, true); err != nil {
		return err
	}

	// If it's a built-in plugin, load it
	m.mu.RLock()
	factory, isBuiltIn := m.builtInFactories[id]
	_, alreadyLoaded := m.plugins[id]
	m.mu.RUnlock()

	if isBuiltIn && !alreadyLoaded {
		plugin := factory()
		if err := m.loadPlugin(ctx, plugin); err != nil {
			return err
		}
		if m.started {
			if err := plugin.Start(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// DisablePlugin disables a plugin
func (m *Manager) DisablePlugin(ctx context.Context, id string) error {
	if err := m.db.SetPluginEnabled(id, false); err != nil {
		return err
	}

	// Stop and unload the plugin
	m.mu.Lock()
	plugin, loaded := m.plugins[id]
	if loaded {
		delete(m.plugins, id)
		// Remove from order
		for i, pid := range m.pluginOrder {
			if pid == id {
				m.pluginOrder = append(m.pluginOrder[:i], m.pluginOrder[i+1:]...)
				break
			}
		}
	}
	m.mu.Unlock()

	if loaded {
		if err := plugin.Stop(ctx); err != nil {
			log.Printf("Error stopping plugin %s: %v", id, err)
		}
	}

	return nil
}

// GetEventBus returns the event bus
func (m *Manager) GetEventBus() *EventBusImpl {
	return m.eventBus
}

// PublishEvent publishes an event to all subscribers
func (m *Manager) PublishEvent(event Event) {
	m.eventBus.Publish(event)
}

// scopedPluginDB provides scoped database access for a specific plugin
type scopedPluginDB struct {
	db       *storage.DB
	pluginID string
}

func (s *scopedPluginDB) Get(key string) ([]byte, error) {
	return s.db.GetPluginData(s.pluginID, key)
}

func (s *scopedPluginDB) Set(key string, value []byte) error {
	return s.db.SetPluginData(s.pluginID, key, value)
}

func (s *scopedPluginDB) Delete(key string) error {
	return s.db.DeletePluginData(s.pluginID, key)
}

func (s *scopedPluginDB) List(prefix string) (map[string][]byte, error) {
	return s.db.ListPluginData(s.pluginID, prefix)
}

func (s *scopedPluginDB) GetSetting(key string) (string, error) {
	return s.db.GetPluginSetting(s.pluginID, key)
}

func (s *scopedPluginDB) SetSetting(key string, value string) error {
	return s.db.SetPluginSetting(s.pluginID, key, value)
}

func (s *scopedPluginDB) GetAllSettings() (map[string]string, error) {
	return s.db.GetAllPluginSettings(s.pluginID)
}
// External Plugin Management Methods

// InstallExternalPlugin installs a plugin from a GitHub repository URL
func (m *Manager) InstallExternalPlugin(ctx context.Context, repoURL, version string) error {
	log.Printf("[PluginManager] Installing external plugin from %s", repoURL)

	// Install the plugin files and save to database
	if err := m.installer.Install(ctx, repoURL, version); err != nil {
		return err
	}

	// Get the plugin ID from the installer
	// The plugin ID is extracted during install, we need to get the plugin record
	plugins, err := m.db.GetAllPlugins()
	if err != nil {
		return fmt.Errorf("failed to get plugins after install: %w", err)
	}

	// Find the newly installed plugin (it will be enabled and from the repo URL)
	var pluginID string
	for _, p := range plugins {
		if p.SourceURL == repoURL && p.Enabled {
			pluginID = p.ID
			break
		}
	}

	if pluginID == "" {
		return fmt.Errorf("could not find installed plugin from %s", repoURL)
	}

	log.Printf("[PluginManager] Starting installed plugin %s", pluginID)

	// Start the plugin process
	if err := m.StartExternalPlugin(ctx, pluginID); err != nil {
		log.Printf("[PluginManager] Failed to start plugin %s: %v", pluginID, err)
		return fmt.Errorf("plugin installed but failed to start: %w", err)
	}

	// Mount routes with retry
	mounted := false
	for attempt := 1; attempt <= 5; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		if err := m.MountPluginRoutes(pluginID); err != nil {
			if attempt < 5 {
				log.Printf("[PluginManager] Attempt %d/5: Failed to mount routes for %s, retrying...", attempt, pluginID)
				continue
			}
			log.Printf("Failed to mount routes for plugin %s after 5 attempts: %v", pluginID, err)
		} else {
			mounted = true
			break
		}
	}

	if mounted {
		// Fetch and save tab configuration
		if err := m.FetchAndSaveTabConfig(ctx, pluginID); err != nil {
			log.Printf("Warning: Failed to fetch tab config for %s: %v", pluginID, err)
		}
	}

	return nil
}

// UpdateExternalPlugin updates an external plugin to the latest version
func (m *Manager) UpdateExternalPlugin(ctx context.Context, pluginID string) error {
	log.Printf("[PluginManager] Updating external plugin %s", pluginID)
	return m.installer.Update(ctx, pluginID)
}

// UninstallExternalPlugin removes an external plugin
func (m *Manager) UninstallExternalPlugin(pluginID string) error {
	log.Printf("[PluginManager] Uninstalling external plugin %s", pluginID)

	// Stop plugin process if running
	if err := m.supervisor.StopPlugin(pluginID); err != nil {
		log.Printf("[PluginManager] Warning: failed to stop plugin %s: %v", pluginID, err)
	}

	// Unregister from Census API
	m.censusAPIServer.UnregisterPlugin(pluginID)

	// Remove plugin files and database record
	return m.installer.Uninstall(pluginID)
}

// GetExternalPluginLogs returns recent log output from a plugin process
func (m *Manager) GetExternalPluginLogs(pluginID string) (stdout, stderr []string, err error) {
	return m.supervisor.GetPluginLogs(pluginID)
}

// GetExternalPluginStatus returns the runtime status of an external plugin
func (m *Manager) GetExternalPluginStatus(pluginID string) (external.PluginStatus, error) {
	return m.supervisor.GetPluginStatus(pluginID)
}

// StartExternalPlugin starts an external plugin process
func (m *Manager) StartExternalPlugin(ctx context.Context, pluginID string) error {
	log.Printf("[PluginManager] Starting external plugin %s", pluginID)

	// Get plugin metadata
	plugin, err := m.db.GetExternalPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("failed to get plugin metadata: %w", err)
	}

	// Register permissions with Census API
	if err := m.censusAPIServer.RegisterPlugin(pluginID, plugin.Permissions); err != nil {
		return fmt.Errorf("failed to register plugin permissions: %w", err)
	}

	// Start plugin process
	if err := m.supervisor.StartPlugin(ctx, pluginID); err != nil {
		m.censusAPIServer.UnregisterPlugin(pluginID)
		return fmt.Errorf("failed to start plugin process: %w", err)
	}

	return nil
}

// StopExternalPlugin stops an external plugin process
func (m *Manager) StopExternalPlugin(pluginID string) error {
	log.Printf("[PluginManager] Stopping external plugin %s", pluginID)

	// Stop plugin process
	if err := m.supervisor.StopPlugin(pluginID); err != nil {
		return err
	}

	// Unregister from Census API
	m.censusAPIServer.UnregisterPlugin(pluginID)

	return nil
}

// FetchAndSaveTabConfig retrieves tab configuration from a plugin via gRPC and saves it to the database
func (m *Manager) FetchAndSaveTabConfig(ctx context.Context, pluginID string) error {
	log.Printf("[PluginManager] Fetching tab config for plugin %s", pluginID)

	// Get the gRPC client from supervisor
	client, err := m.supervisor.GetGRPCClient(pluginID)
	if err != nil {
		return fmt.Errorf("failed to get plugin gRPC client: %w", err)
	}

	// Call GetTab via gRPC with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.GetTab(ctx, &pb.TabRequest{
		PluginId: pluginID,
	})

	if err != nil {
		return fmt.Errorf("failed to call GetTab: %w", err)
	}

	// If plugin doesn't have a tab, skip
	if !resp.HasTab {
		log.Printf("[PluginManager] Plugin %s does not provide a tab", pluginID)
		return nil
	}

	// Create TabDefinition structure
	tabDef := TabDefinition{
		ID:        resp.Id,
		Label:     resp.Label,
		Icon:      resp.Icon,
		Order:     int(resp.Order),
		ScriptURL: resp.ScriptUrl,
		InitFunc:  resp.InitFunc,
	}

	// Get existing plugin record
	plugin, err := m.db.GetExternalPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("failed to get plugin record: %w", err)
	}

	// Update tab_config field as map[string]string
	plugin.TabConfig = map[string]string{
		"id":         tabDef.ID,
		"label":      tabDef.Label,
		"icon":       tabDef.Icon,
		"order":      fmt.Sprintf("%d", tabDef.Order),
		"script_url": tabDef.ScriptURL,
		"init_func":  tabDef.InitFunc,
	}

	// Save back to database
	if err := m.db.SaveExternalPlugin(plugin); err != nil {
		return fmt.Errorf("failed to save tab config: %w", err)
	}

	log.Printf("[PluginManager] Successfully saved tab config for %s: %s", pluginID, tabDef.Label)
	return nil
}

// MountPluginRoutes dynamically mounts routes for an external plugin
func (m *Manager) MountPluginRoutes(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.router == nil {
		return fmt.Errorf("router not set")
	}

	// Get gRPC client for the plugin
	client, err := m.supervisor.GetGRPCClient(pluginID)
	if err != nil {
		return fmt.Errorf("failed to get plugin gRPC client: %w", err)
	}

	// Create a subrouter for this plugin under /api/p/{pluginID}/*
	pluginPath := fmt.Sprintf("/p/%s", pluginID)
	pluginRouter := m.router.PathPrefix(pluginPath).Subrouter()

	// Mount a catch-all handler that forwards requests to the plugin via gRPC
	pluginRouter.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.handlePluginRoute(w, r, pluginID, client)
	})

	log.Printf("[PluginManager] Mounted routes for plugin %s at /api%s", pluginID, pluginPath)
	return nil
}

// handlePluginRoute forwards HTTP requests to an external plugin via gRPC
func (m *Manager) handlePluginRoute(w http.ResponseWriter, r *http.Request, pluginID string, client pb.PluginClient) {
	// Extract path after /api/p/{pluginID}/
	pathPrefix := fmt.Sprintf("/api/p/%s/", pluginID)
	pluginPath := r.URL.Path
	if len(pluginPath) >= len(pathPrefix) {
		pluginPath = pluginPath[len(pathPrefix)-1:] // Keep the leading slash
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	// Convert headers to map
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Convert query parameters to map
	queryParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	// Call plugin via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.HandleRoute(ctx, &pb.RouteRequest{
		PluginId:    pluginID,
		Method:      r.Method,
		Path:        pluginPath,
		Headers:     headers,
		Body:        body,
		QueryParams: queryParams,
	})

	if err != nil {
		log.Printf("[PluginManager] Plugin %s route handler error: %v", pluginID, err)
		http.Error(w, "Plugin error", http.StatusInternalServerError)
		return
	}

	// Set response headers
	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}

	// Set status code
	w.WriteHeader(int(resp.StatusCode))

	// Write response body
	if len(resp.Body) > 0 {
		w.Write(resp.Body)
	}
}
