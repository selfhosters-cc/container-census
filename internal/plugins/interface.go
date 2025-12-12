package plugins

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/selfhosters-cc/container-census/internal/models"
)

// Plugin represents a Container Census plugin
type Plugin interface {
	// Info returns plugin metadata
	Info() PluginInfo

	// Lifecycle methods
	Init(ctx context.Context, deps PluginDependencies) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Capabilities - return nil if not supported
	Routes() []Route                              // API routes under /api/plugins/{id}/
	Tab() *TabDefinition                          // UI tab (appears in Integrations dropdown)
	Badges() []BadgeProvider                      // Badges on container cards
	ContainerEnricher() ContainerEnricher         // Enrich container data
	Settings() *SettingsDefinition                // Settings schema
	NotificationChannelFactory() ChannelFactory   // Notification channel factory
}

// PluginInfo contains plugin metadata
type PluginInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Author       string   `json:"author"`
	Homepage     string   `json:"homepage,omitempty"`
	Capabilities []string `json:"capabilities"` // data_source, ui_tab, ui_badge, notification_channel, settings
	BuiltIn      bool     `json:"built_in"`
}

// PluginDependencies provides access to core Census features
type PluginDependencies struct {
	DB         PluginDB          // Database access (scoped to plugin prefix)
	Containers ContainerProvider // Access to container data
	Hosts      HostProvider      // Access to host data
	HTTPClient *http.Client      // Pre-configured HTTP client
	Logger     PluginLogger      // Scoped logger
	EventBus   EventBus          // Subscribe to system events
}

// PluginDB provides scoped database access for plugins
type PluginDB interface {
	// Get retrieves a value by key
	Get(key string) ([]byte, error)
	// Set stores a value by key
	Set(key string, value []byte) error
	// Delete removes a key
	Delete(key string) error
	// List returns all key-value pairs with the given prefix
	List(prefix string) (map[string][]byte, error)
	// GetSetting retrieves a setting value
	GetSetting(key string) (string, error)
	// SetSetting stores a setting value
	SetSetting(key string, value string) error
	// GetAllSettings retrieves all settings
	GetAllSettings() (map[string]string, error)
}

// ContainerProvider provides read access to container data
type ContainerProvider interface {
	// GetContainers returns all containers from the latest scan
	GetContainers() []models.Container
	// GetContainerByID returns a specific container
	GetContainerByID(hostID int64, containerID string) (*models.Container, error)
}

// HostProvider provides read access to host data
type HostProvider interface {
	// GetHosts returns all configured hosts
	GetHosts() ([]models.Host, error)
	// GetHostByID returns a specific host
	GetHostByID(id int64) (*models.Host, error)
}

// PluginLogger provides scoped logging for plugins
type PluginLogger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// Route defines an API route provided by a plugin
type Route struct {
	Path    string           // Path relative to /api/plugins/{plugin_id}/
	Method  string           // HTTP method (GET, POST, PUT, DELETE)
	Handler http.HandlerFunc // Handler function
}

// TabDefinition defines a UI tab provided by a plugin
type TabDefinition struct {
	ID          string `json:"id"`                      // Unique tab ID (used in URL hash)
	Label       string `json:"label"`                   // Display label
	Icon        string `json:"icon"`                    // Emoji or icon
	Order       int    `json:"order"`                   // Sort order (lower = first)
	ContentHTML string `json:"content_html"`            // Initial HTML content
	ScriptJS    string `json:"script_js"`               // JavaScript code for tab (inline)
	ScriptURL   string `json:"script_url,omitempty"`    // URL to external JavaScript file
	InitFunc    string `json:"init_func,omitempty"`     // Function name to call after loading script
	StyleCSS    string `json:"style_css"`               // CSS styles for tab
}

// Badge represents a visual badge on a container card
type Badge struct {
	ID       string `json:"id"`
	PluginID string `json:"plugin_id"`
	Label    string `json:"label"`
	Icon     string `json:"icon"`
	Color    string `json:"color"`    // success, warning, danger, info, secondary
	Tooltip  string `json:"tooltip"`
	Link     string `json:"link,omitempty"`
	Priority int    `json:"priority"` // Higher = shown first
}

// BadgeProvider generates badges for containers
type BadgeProvider interface {
	// GetBadge returns a badge for the container, or nil if not applicable
	GetBadge(ctx context.Context, container models.Container) (*Badge, error)
	// GetBadgeID returns a unique identifier for this badge provider
	GetBadgeID() string
}

// ContainerEnricher adds data to container objects
type ContainerEnricher interface {
	// Enrich adds plugin-specific data to the container's PluginData map
	Enrich(ctx context.Context, container *models.Container) error
	// GetEnrichmentKey returns the key used in PluginData map
	GetEnrichmentKey() string
}

// SettingsDefinition defines the settings schema for a plugin
type SettingsDefinition struct {
	Fields []SettingsField `json:"fields"`
}

// SettingsField defines a single setting field
type SettingsField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"` // text, password, number, boolean, select
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required"`
	Options     []Option `json:"options,omitempty"` // For select type
	Min         *int     `json:"min,omitempty"`     // For number type
	Max         *int     `json:"max,omitempty"`     // For number type
}

// Option represents a select option
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ChannelFactory creates notification channel instances
type ChannelFactory interface {
	// CreateChannel creates a notification channel from config
	CreateChannel(config map[string]interface{}) (NotificationChannel, error)
	// GetType returns the channel type name
	GetType() string
	// GetConfigSchema returns the configuration schema
	GetConfigSchema() []SettingsField
}

// NotificationChannel sends notifications
type NotificationChannel interface {
	Send(ctx context.Context, message string, event models.NotificationEvent) error
	Test(ctx context.Context) error
}

// EventBus allows plugins to subscribe to system events
type EventBus interface {
	// Subscribe registers a handler for an event type
	// Returns an unsubscribe function
	Subscribe(eventType string, handler EventHandler) func()
	// Publish sends an event to all subscribers
	Publish(event Event)
}

// EventHandler handles events
type EventHandler func(ctx context.Context, event Event)

// Event represents a system event
type Event struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Common event types
const (
	EventScanComplete         = "scan_complete"
	EventContainerStateChange = "container_state_change"
	EventContainerCreated     = "container_created"
	EventContainerRemoved     = "container_removed"
	EventImageUpdated         = "image_updated"
	EventHostAdded            = "host_added"
	EventHostRemoved          = "host_removed"
)

// DefaultPluginLogger provides a simple logger implementation
type DefaultPluginLogger struct {
	Prefix string
}

func (l *DefaultPluginLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] [%s] %s %v", l.Prefix, msg, args)
}

func (l *DefaultPluginLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] [%s] %s %v", l.Prefix, msg, args)
}

func (l *DefaultPluginLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] [%s] %s %v", l.Prefix, msg, args)
}

func (l *DefaultPluginLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] [%s] %s %v", l.Prefix, msg, args)
}
