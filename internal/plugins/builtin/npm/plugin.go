package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/selfhosters-cc/container-census/internal/models"
	"github.com/selfhosters-cc/container-census/internal/plugins"
	"github.com/gorilla/mux"
)

// Plugin implements the NPM (Nginx Proxy Manager) integration
type Plugin struct {
	mu         sync.RWMutex
	deps       plugins.PluginDependencies
	instances  map[int64]*NPMInstance
	proxyHosts map[int64][]ProxyHost // instanceID -> proxy hosts
	mappings   map[string][]ProxyHostMapping // containerKey -> proxy host mappings
	stopChan   chan struct{}
	syncTicker *time.Ticker
}

// NPMInstance represents a configured NPM instance
type NPMInstance struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Email     string    `json:"email"`
	Password  string    `json:"password,omitempty"` // Omit when empty, manually excluded in GET responses
	Enabled   bool      `json:"enabled"`
	LastSync  time.Time `json:"last_sync,omitempty"`
	LastError string    `json:"last_error,omitempty"`
	client    *Client
}

// ProxyHostMapping maps a container to an NPM proxy host
type ProxyHostMapping struct {
	InstanceID   int64    `json:"instance_id"`
	InstanceName string   `json:"instance_name"`
	ProxyHostID  int      `json:"proxy_host_id"`
	DomainNames  []string `json:"domain_names"`
	SSLEnabled   bool     `json:"ssl_enabled"`
	Enabled      bool     `json:"enabled"`
	MatchType    string   `json:"match_type"` // "ip_port" or "hostname"
}

// New creates a new NPM plugin instance
func New() plugins.Plugin {
	return &Plugin{
		instances:  make(map[int64]*NPMInstance),
		proxyHosts: make(map[int64][]ProxyHost),
		mappings:   make(map[string][]ProxyHostMapping),
		stopChan:   make(chan struct{}),
	}
}

// Register registers the NPM plugin with the plugin manager
func Register(manager *plugins.Manager) {
	manager.RegisterBuiltIn("npm", New)
}

// Info returns plugin metadata
func (p *Plugin) Info() plugins.PluginInfo {
	return plugins.PluginInfo{
		ID:          "npm",
		Name:        "Nginx Proxy Manager",
		Description: "Integration with Nginx Proxy Manager to show which containers are exposed externally",
		Version:     "1.0.0",
		Author:      "Container Census",
		Capabilities: []string{
			"data_source",
			"ui_tab",
			"ui_badge",
			"settings",
		},
		BuiltIn: true,
	}
}

// Init initializes the plugin
func (p *Plugin) Init(ctx context.Context, deps plugins.PluginDependencies) error {
	p.deps = deps

	// Load instances from storage
	if err := p.loadInstances(); err != nil {
		log.Printf("NPM plugin: failed to load instances: %v", err)
	}

	return nil
}

// Start starts the plugin background tasks
func (p *Plugin) Start(ctx context.Context) error {
	// Initial sync
	p.syncAllInstances()

	// Start periodic sync (every 5 minutes)
	p.syncTicker = time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-p.syncTicker.C:
				p.syncAllInstances()
			case <-p.stopChan:
				return
			}
		}
	}()

	return nil
}

// Stop stops the plugin
func (p *Plugin) Stop(ctx context.Context) error {
	if p.syncTicker != nil {
		p.syncTicker.Stop()
	}
	close(p.stopChan)
	return nil
}

// Routes returns the plugin's API routes
func (p *Plugin) Routes() []plugins.Route {
	return []plugins.Route{
		{Path: "/instances", Method: "GET", Handler: p.handleGetInstances},
		{Path: "/instances", Method: "POST", Handler: p.handleAddInstance},
		{Path: "/instances/{id}", Method: "GET", Handler: p.handleGetInstance},
		{Path: "/instances/{id}", Method: "PUT", Handler: p.handleUpdateInstance},
		{Path: "/instances/{id}", Method: "DELETE", Handler: p.handleDeleteInstance},
		{Path: "/instances/{id}/test", Method: "POST", Handler: p.handleTestInstance},
		{Path: "/instances/{id}/sync", Method: "POST", Handler: p.handleSyncInstance},
		{Path: "/proxy-hosts", Method: "GET", Handler: p.handleGetProxyHosts},
		{Path: "/exposed", Method: "GET", Handler: p.handleGetExposed},
		{Path: "/tab", Method: "GET", Handler: p.handleGetTab},
	}
}

// Tab returns the tab definition for the plugin
func (p *Plugin) Tab() *plugins.TabDefinition {
	return &plugins.TabDefinition{
		ID:        "npm",
		Label:     "Nginx Proxy Manager",
		Icon:      "🌐",
		Order:     100,
		ScriptURL: "/plugins/npm.js",
		InitFunc:  "npmPluginInit",
	}
}

// Badges returns badge providers
func (p *Plugin) Badges() []plugins.BadgeProvider {
	return []plugins.BadgeProvider{p}
}

// GetBadge returns a badge for a container if it's exposed via NPM
func (p *Plugin) GetBadge(ctx context.Context, container models.Container) (*plugins.Badge, error) {
	containerKey := fmt.Sprintf("%d-%s", container.HostID, container.ID)

	p.mu.RLock()
	mappings, exists := p.mappings[containerKey]
	p.mu.RUnlock()

	if !exists || len(mappings) == 0 {
		return nil, nil
	}

	// Get the first (primary) mapping
	mapping := mappings[0]
	domain := ""
	if len(mapping.DomainNames) > 0 {
		domain = mapping.DomainNames[0]
	}

	badge := &plugins.Badge{
		ID:       "npm-exposed",
		Label:    domain,
		Icon:     "🌐",
		Color:    "info",
		Priority: 100,
		Tooltip:  fmt.Sprintf("Exposed via %s", mapping.InstanceName),
	}

	if domain != "" {
		scheme := "http"
		if mapping.SSLEnabled {
			scheme = "https"
		}
		badge.Link = fmt.Sprintf("%s://%s", scheme, domain)
	}

	return badge, nil
}

// GetBadgeID returns a unique identifier for this badge provider
func (p *Plugin) GetBadgeID() string {
	return "npm-exposed"
}

// ContainerEnricher returns the container enricher
func (p *Plugin) ContainerEnricher() plugins.ContainerEnricher {
	return p
}

// Enrich adds NPM data to a container
func (p *Plugin) Enrich(ctx context.Context, container *models.Container) error {
	containerKey := fmt.Sprintf("%d-%s", container.HostID, container.ID)

	p.mu.RLock()
	mappings, exists := p.mappings[containerKey]
	p.mu.RUnlock()

	if !exists || len(mappings) == 0 {
		return nil
	}

	if container.PluginData == nil {
		container.PluginData = make(map[string]interface{})
	}

	container.PluginData["npm"] = map[string]interface{}{
		"exposed":  true,
		"mappings": mappings,
	}

	return nil
}

// GetEnrichmentKey returns the key used in PluginData map
func (p *Plugin) GetEnrichmentKey() string {
	return "npm"
}

// Settings returns the settings definition
func (p *Plugin) Settings() *plugins.SettingsDefinition {
	return &plugins.SettingsDefinition{
		Fields: []plugins.SettingsField{
			{
				Key:         "sync_interval",
				Label:       "Sync Interval (minutes)",
				Type:        "number",
				Default:     "5",
				Description: "How often to sync with NPM instances",
			},
		},
	}
}

// NotificationChannelFactory returns nil (NPM doesn't provide notification channels)
func (p *Plugin) NotificationChannelFactory() plugins.ChannelFactory {
	return nil
}

// loadInstances loads NPM instances from storage
func (p *Plugin) loadInstances() error {
	data, err := p.deps.DB.List("instances/")
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, v := range data {
		var inst NPMInstance
		if err := json.Unmarshal(v, &inst); err != nil {
			continue
		}
		inst.client = NewClient(inst.URL, inst.Email, inst.Password)
		p.instances[inst.ID] = &inst
	}

	return nil
}

// saveInstance saves an NPM instance to storage
func (p *Plugin) saveInstance(inst *NPMInstance) error {
	data, err := json.Marshal(inst)
	if err != nil {
		return err
	}
	return p.deps.DB.Set(fmt.Sprintf("instances/%d", inst.ID), data)
}

// deleteInstance removes an NPM instance from storage
func (p *Plugin) deleteInstance(id int64) error {
	return p.deps.DB.Delete(fmt.Sprintf("instances/%d", id))
}

// syncAllInstances syncs all enabled NPM instances
func (p *Plugin) syncAllInstances() {
	p.mu.RLock()
	instances := make([]*NPMInstance, 0, len(p.instances))
	for _, inst := range p.instances {
		if inst.Enabled {
			instances = append(instances, inst)
		}
	}
	p.mu.RUnlock()

	for _, inst := range instances {
		p.syncInstance(inst)
	}

	// Rebuild container mappings
	p.rebuildMappings()
}

// syncInstance syncs a single NPM instance
func (p *Plugin) syncInstance(inst *NPMInstance) {
	if inst.client == nil {
		inst.client = NewClient(inst.URL, inst.Email, inst.Password)
	}

	hosts, err := inst.client.GetProxyHosts()
	if err != nil {
		p.mu.Lock()
		inst.LastError = err.Error()
		p.mu.Unlock()
		log.Printf("NPM plugin: failed to sync instance %s: %v", inst.Name, err)
		return
	}

	p.mu.Lock()
	p.proxyHosts[inst.ID] = hosts
	inst.LastSync = time.Now()
	inst.LastError = ""
	p.mu.Unlock()

	if err := p.saveInstance(inst); err != nil {
		log.Printf("NPM plugin: failed to save instance state: %v", err)
	}

	log.Printf("NPM plugin: synced %d proxy hosts from %s", len(hosts), inst.Name)
}

// rebuildMappings rebuilds container-to-proxy-host mappings
func (p *Plugin) rebuildMappings() {
	containers := p.deps.Containers.GetContainers()

	newMappings := make(map[string][]ProxyHostMapping)

	p.mu.RLock()
	for instanceID, hosts := range p.proxyHosts {
		inst := p.instances[instanceID]
		if inst == nil {
			continue
		}

		for _, host := range hosts {
			for _, container := range containers {
				if p.matchContainer(container, host) {
					containerKey := fmt.Sprintf("%d-%s", container.HostID, container.ID)
					mapping := ProxyHostMapping{
						InstanceID:   instanceID,
						InstanceName: inst.Name,
						ProxyHostID:  host.ID,
						DomainNames:  host.DomainNames,
						SSLEnabled:   host.CertificateID > 0,
						Enabled:      host.Enabled,
						MatchType:    "ip_port",
					}
					newMappings[containerKey] = append(newMappings[containerKey], mapping)
				}
			}
		}
	}
	p.mu.RUnlock()

	p.mu.Lock()
	p.mappings = newMappings
	p.mu.Unlock()
}

// matchContainer checks if a container matches an NPM proxy host
func (p *Plugin) matchContainer(container models.Container, host ProxyHost) bool {
	// Match by container IP + private port (Docker bridge network)
	for _, nd := range container.NetworkDetails {
		for _, port := range container.Ports {
			if nd.IPAddress == host.ForwardHost && port.PrivatePort == host.ForwardPort {
				return true
			}
		}
	}

	// Get the host's IP address(es) from the agent or local host
	// This requires access to the host information
	hostObj, err := p.deps.Hosts.GetHostByID(container.HostID)
	if err == nil && hostObj != nil {
		// Extract IP from address (e.g., "http://192.168.5.3:9876" -> "192.168.5.3")
		hostIP := extractIPFromAddress(hostObj.Address)

		// Match by host IP + public port (when NPM forwards to host:port)
		if hostIP == host.ForwardHost {
			for _, port := range container.Ports {
				if port.PublicPort == host.ForwardPort {
					return true
				}
			}
		}
	}

	// Match by container name (if NPM uses Docker DNS)
	if host.ForwardHost == container.Name {
		return true
	}

	// Match by container name prefix (common pattern: container_name or containername)
	containerNameClean := container.Name
	if len(containerNameClean) > 0 && containerNameClean[0] == '/' {
		containerNameClean = containerNameClean[1:]
	}
	if host.ForwardHost == containerNameClean {
		return true
	}

	return false
}

// extractIPFromAddress extracts the IP address from various address formats
func extractIPFromAddress(address string) string {
	// Handle formats like:
	// - "http://192.168.5.3:9876" -> "192.168.5.3"
	// - "agent://192.168.5.3:9876" -> "192.168.5.3"
	// - "192.168.5.3" -> "192.168.5.3"
	// - "unix:///var/run/docker.sock" -> "" (not an IP)

	// Remove protocol prefix
	address = strings.TrimPrefix(address, "http://")
	address = strings.TrimPrefix(address, "https://")
	address = strings.TrimPrefix(address, "agent://")
	address = strings.TrimPrefix(address, "tcp://")

	// If it's a unix socket, return empty
	if strings.HasPrefix(address, "unix://") {
		return ""
	}

	// Split by colon to remove port
	if idx := strings.Index(address, ":"); idx != -1 {
		address = address[:idx]
	}

	return address
}

// HTTP Handlers

func (p *Plugin) handleGetInstances(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	type InstanceResponse struct {
		ID             int64     `json:"id"`
		Name           string    `json:"name"`
		URL            string    `json:"url"`
		Email          string    `json:"email"`
		Enabled        bool      `json:"enabled"`
		LastSync       time.Time `json:"last_sync,omitempty"`
		LastError      string    `json:"last_error,omitempty"`
		ProxyHostCount int       `json:"proxy_host_count"`
	}

	instances := make([]*InstanceResponse, 0, len(p.instances))
	for _, inst := range p.instances {
		// Count proxy hosts for this instance
		proxyHostCount := len(p.proxyHosts[inst.ID])

		// Don't include password
		safe := &InstanceResponse{
			ID:             inst.ID,
			Name:           inst.Name,
			URL:            inst.URL,
			Email:          inst.Email,
			Enabled:        inst.Enabled,
			LastSync:       inst.LastSync,
			LastError:      inst.LastError,
			ProxyHostCount: proxyHostCount,
		}
		instances = append(instances, safe)
	}
	p.mu.RUnlock()

	writeJSON(w, instances)
}

func (p *Plugin) handleAddInstance(w http.ResponseWriter, r *http.Request) {
	var inst NPMInstance
	if err := json.NewDecoder(r.Body).Decode(&inst); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate ID
	p.mu.Lock()
	maxID := int64(0)
	for id := range p.instances {
		if id > maxID {
			maxID = id
		}
	}
	inst.ID = maxID + 1
	inst.Enabled = true
	inst.client = NewClient(inst.URL, inst.Email, inst.Password)
	p.instances[inst.ID] = &inst
	p.mu.Unlock()

	if err := p.saveInstance(&inst); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Sync the new instance
	go func() {
		p.syncInstance(&inst)
		p.rebuildMappings()
	}()

	writeJSON(w, map[string]interface{}{"id": inst.ID})
}

func (p *Plugin) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	instID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "invalid instance ID", http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	inst, exists := p.instances[instID]
	p.mu.RUnlock()

	if !exists {
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}

	safe := &NPMInstance{
		ID:        inst.ID,
		Name:      inst.Name,
		URL:       inst.URL,
		Email:     inst.Email,
		Enabled:   inst.Enabled,
		LastSync:  inst.LastSync,
		LastError: inst.LastError,
	}
	writeJSON(w, safe)
}

func (p *Plugin) handleUpdateInstance(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	instID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "invalid instance ID", http.StatusBadRequest)
		return
	}

	p.mu.Lock()
	inst, exists := p.instances[instID]
	if !exists {
		p.mu.Unlock()
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}

	var update NPMInstance
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		p.mu.Unlock()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	inst.Name = update.Name
	inst.URL = update.URL
	inst.Email = update.Email
	if update.Password != "" {
		inst.Password = update.Password
	}
	inst.Enabled = update.Enabled
	inst.client = NewClient(inst.URL, inst.Email, inst.Password)
	p.mu.Unlock()

	if err := p.saveInstance(inst); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (p *Plugin) handleDeleteInstance(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	instID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "invalid instance ID", http.StatusBadRequest)
		return
	}

	p.mu.Lock()
	delete(p.instances, instID)
	delete(p.proxyHosts, instID)
	p.mu.Unlock()

	if err := p.deleteInstance(instID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.rebuildMappings()
	w.WriteHeader(http.StatusOK)
}

func (p *Plugin) handleTestInstance(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	instID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "invalid instance ID", http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	inst, exists := p.instances[instID]
	p.mu.RUnlock()

	if !exists {
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}

	client := NewClient(inst.URL, inst.Email, inst.Password)
	if err := client.TestConnection(); err != nil {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
	})
}

func (p *Plugin) handleSyncInstance(w http.ResponseWriter, r *http.Request) {
	id := getPathParam(r, "id")
	instID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "invalid instance ID", http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	inst, exists := p.instances[instID]
	p.mu.RUnlock()

	if !exists {
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}

	p.syncInstance(inst)
	p.rebuildMappings()

	p.mu.RLock()
	hostCount := len(p.proxyHosts[instID])
	lastError := inst.LastError
	p.mu.RUnlock()

	writeJSON(w, map[string]interface{}{
		"success":    lastError == "",
		"host_count": hostCount,
		"error":      lastError,
	})
}

func (p *Plugin) handleGetProxyHosts(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	allHosts := make([]map[string]interface{}, 0)
	containers := p.deps.Containers.GetContainers()

	for instanceID, hosts := range p.proxyHosts {
		inst := p.instances[instanceID]
		for _, host := range hosts {
			result := map[string]interface{}{
				"instance_id":   instanceID,
				"instance_name": inst.Name,
				"host":          host,
			}

			// Try to find a matching container for this proxy host
			for _, container := range containers {
				if p.matchContainer(container, host) {
					result["container_name"] = container.Name
					result["host_name"] = container.HostName
					break // Only match the first container found
				}
			}

			allHosts = append(allHosts, result)
		}
	}

	writeJSON(w, allHosts)
}

func (p *Plugin) handleGetExposed(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	exposed := make([]map[string]interface{}, 0)
	for containerKey, mappings := range p.mappings {
		exposed = append(exposed, map[string]interface{}{
			"container_key": containerKey,
			"mappings":      mappings,
		})
	}

	writeJSON(w, exposed)
}

func (p *Plugin) handleGetTab(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(npmTabHTML))
}

// Helper functions

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func getPathParam(r *http.Request, name string) string {
	vars := mux.Vars(r)
	return vars[name]
}

// Tab HTML template - JavaScript is loaded from /plugins/npm.js
const npmTabHTML = `
<div class="tab-header">
    <h2>Nginx Proxy Manager Integration</h2>
    <div class="tab-actions">
        <button class="btn btn-primary" id="npmAddInstanceBtn">+ Add Instance</button>
    </div>
</div>

<div class="npm-content">
    <!-- Instances Section -->
    <div class="section">
        <h3>NPM Instances</h3>
        <div id="npmInstances" class="npm-instances-grid">
            <div class="loading">Loading instances...</div>
        </div>
    </div>

    <!-- Exposed Services Section -->
    <div class="section">
        <h3>Exposed Services</h3>
        <div id="npmExposed" class="npm-exposed-table">
            <div class="loading">Loading exposed services...</div>
        </div>
    </div>
</div>

<!-- Add/Edit Instance Modal -->
<div id="npmInstanceModal" class="modal" style="display: none;">
    <div class="modal-content">
        <div class="modal-header">
            <h3 id="npmModalTitle">Add NPM Instance</h3>
            <button class="modal-close" id="npmCloseModalBtn">&times;</button>
        </div>
        <form id="npmInstanceForm">
            <input type="hidden" id="npmInstanceId">
            <div class="form-group">
                <label for="npmInstanceName">Name</label>
                <input type="text" id="npmInstanceName" required placeholder="My NPM">
            </div>
            <div class="form-group">
                <label for="npmInstanceUrl">URL</label>
                <input type="url" id="npmInstanceUrl" required placeholder="http://npm.local:81">
            </div>
            <div class="form-group">
                <label for="npmInstanceEmail">Email</label>
                <input type="email" id="npmInstanceEmail" required placeholder="admin@example.com">
            </div>
            <div class="form-group">
                <label for="npmInstancePassword">Password</label>
                <input type="password" id="npmInstancePassword" placeholder="Leave blank to keep existing">
            </div>
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" id="npmCancelBtn">Cancel</button>
                <button type="submit" class="btn btn-primary">Save</button>
            </div>
        </form>
    </div>
</div>

<style>
.npm-content {
    padding: 20px;
}

.npm-instances-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
    gap: 20px;
}

.npm-instance-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 16px;
}

.npm-instance-card .instance-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
}

.npm-instance-card .instance-header h4 {
    margin: 0;
    font-size: 1.1rem;
}

.npm-instance-card .instance-details {
    font-size: 0.9rem;
    color: var(--text-secondary);
}

.npm-instance-card .instance-details .detail {
    margin: 4px 0;
}

.npm-instance-card .instance-details .label {
    font-weight: 500;
    color: var(--text-primary);
}

.npm-instance-card .instance-details .error {
    color: var(--danger);
}

.npm-instance-card .instance-actions {
    display: flex;
    gap: 8px;
    margin-top: 12px;
    flex-wrap: wrap;
}

.status-badge {
    padding: 4px 8px;
    border-radius: 4px;
    font-size: 0.8rem;
    font-weight: 500;
}

.status-badge.success {
    background: rgba(16, 185, 129, 0.15);
    color: var(--success);
}

.status-badge.error {
    background: rgba(239, 68, 68, 0.15);
    color: var(--danger);
}

.npm-exposed-table {
    overflow-x: auto;
}

.empty-state {
    text-align: center;
    padding: 40px;
    color: var(--text-tertiary);
}

.badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 0.75rem;
    background: var(--bg-tertiary);
}

.badge.success {
    background: rgba(16, 185, 129, 0.15);
    color: var(--success);
}
</style>
`
