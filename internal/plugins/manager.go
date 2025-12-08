package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/container-census/container-census/internal/models"
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
}

// PluginFactory creates a new plugin instance
type PluginFactory func() Plugin

// NewManager creates a new plugin manager
func NewManager(db *storage.DB, containers ContainerProvider, hosts HostProvider) *Manager {
	return &Manager{
		plugins:          make(map[string]Plugin),
		pluginOrder:      make([]string, 0),
		builtInFactories: make(map[string]PluginFactory),
		db:               db,
		containers:       containers,
		hosts:            hosts,
		eventBus:         NewEventBus(),
	}
}

// SetRouter sets the router for mounting plugin routes
func (m *Manager) SetRouter(router *mux.Router) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.router = router
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

// GetRawDB returns the underlying storage.DB for built-in plugins that need full database access
// This should only be used by trusted built-in plugins like the security plugin
func (s *scopedPluginDB) GetRawDB() *storage.DB {
	return s.db
}

func (s *scopedPluginDB) GetAllSettings() (map[string]string, error) {
	return s.db.GetAllPluginSettings(s.pluginID)
}
