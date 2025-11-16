package models

import (
	"fmt"
	"time"
)

// Host represents a Docker host to scan
type Host struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Address      string    `json:"address"`      // e.g., "tcp://host:2376", "ssh://user@host", "agent://host:9876"
	Description  string    `json:"description"`
	HostType     string    `json:"host_type"`    // unix, tcp, ssh, agent
	AgentToken   string    `json:"agent_token,omitempty"` // API token for agent authentication
	AgentStatus  string    `json:"agent_status,omitempty"` // online, offline, unknown
	LastSeen     time.Time `json:"last_seen,omitempty"`
	Enabled      bool      `json:"enabled"`
	CollectStats bool      `json:"collect_stats"` // whether to collect CPU/memory stats for this host
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Container represents a Docker container found on a host
type Container struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	ImageID      string            `json:"image_id"`
	ImageTags    []string          `json:"image_tags"`    // all tags for this image (e.g., ["nginx:1.25", "nginx:latest"])
	ImageSize    int64             `json:"image_size"`    // bytes
	State        string            `json:"state"`         // running, exited, paused, etc.
	Status       string            `json:"status"`        // detailed status
	RestartCount int               `json:"restart_count"` // number of restarts
	Ports        []PortMapping     `json:"ports"`
	Labels       map[string]string `json:"labels"`
	Created      time.Time         `json:"created"`
	HostID       int64             `json:"host_id"`
	HostName     string            `json:"host_name"`
	ScannedAt    time.Time         `json:"scanned_at"`
	// Resource usage stats (may be zero if not collected or if container is idle)
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   int64   `json:"memory_usage"`   // bytes
	MemoryLimit   int64   `json:"memory_limit"`   // bytes
	MemoryPercent float64 `json:"memory_percent"`
	// Connection information for graph visualization
	Networks       []string      `json:"networks,omitempty"`        // Network names this container is connected to
	Volumes        []VolumeMount `json:"volumes,omitempty"`         // Volume mounts
	Links          []string      `json:"links,omitempty"`           // Container links (legacy)
	ComposeProject string        `json:"compose_project,omitempty"` // Docker Compose project name
	// Image update tracking
	UpdateAvailable   bool      `json:"update_available"`
	LastUpdateCheck   time.Time `json:"last_update_check,omitempty"`
}

// PortMapping represents a container port mapping
type PortMapping struct {
	PrivatePort int    `json:"private_port"`
	PublicPort  int    `json:"public_port,omitempty"`
	Type        string `json:"type"` // tcp or udp
	IP          string `json:"ip,omitempty"`
}

// VolumeMount represents a volume or bind mount in a container
type VolumeMount struct {
	Name        string `json:"name"`        // Named volume or source path for bind mounts
	Destination string `json:"destination"` // Mount point inside container
	Type        string `json:"type"`        // "volume", "bind", "tmpfs"
	RW          bool   `json:"rw"`          // Read-write vs read-only
}

// ContainerGraphNode represents a container node in the connection graph
type ContainerGraphNode struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	NodeType       string `json:"node_type"`         // "container" or "network"
	Image          string `json:"image,omitempty"`
	State          string `json:"state,omitempty"`
	HostID         int64  `json:"host_id"`
	HostName       string `json:"host_name,omitempty"`
	ComposeProject string `json:"compose_project,omitempty"`
}

// ContainerGraphEdge represents a connection between containers
type ContainerGraphEdge struct {
	Source string `json:"source"` // Source container ID
	Target string `json:"target"` // Target container ID
	Type   string `json:"type"`   // "network", "volume", "link", "compose"
	Label  string `json:"label"`  // Human-readable label (network name, volume name, etc.)
}

// ContainerGraph represents the complete connection graph
type ContainerGraph struct {
	Nodes []ContainerGraphNode `json:"nodes"`
	Edges []ContainerGraphEdge `json:"edges"`
}

// ScanResult represents a scan operation
type ScanResult struct {
	ID              int64     `json:"id"`
	HostID          int64     `json:"host_id"`
	HostName        string    `json:"host_name"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	Success         bool      `json:"success"`
	Error           string    `json:"error,omitempty"`
	ContainersFound int       `json:"containers_found"`
}

// TelemetrySubmission represents a telemetry submission operation
type TelemetrySubmission struct {
	ID              int64     `json:"id"`
	EndpointName    string    `json:"endpoint_name"`
	EndpointURL     string    `json:"endpoint_url"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	Success         bool      `json:"success"`
	Error           string    `json:"error,omitempty"`
	HostsCount      int       `json:"hosts_count"`
	ContainersCount int       `json:"containers_count"`
	ImagesCount     int       `json:"images_count"`
}

// ActivityLogEntry represents a unified activity log entry (scan or telemetry)
type ActivityLogEntry struct {
	Type      string                 `json:"type"`       // "scan" or "telemetry"
	Timestamp time.Time              `json:"timestamp"`  // started_at
	Target    string                 `json:"target"`     // host_name or endpoint_name
	Duration  float64                `json:"duration"`   // seconds
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"` // Type-specific details
}

// Config represents application configuration
type Config struct {
	Database       DatabaseConfig        `yaml:"database"`
	Server         ServerConfig          `yaml:"server"`
	Scanner        ScannerConfig         `yaml:"scanner"`
	Vulnerability  VulnerabilityConfig   `yaml:"vulnerability"`
	Telemetry      TelemetryConfig       `yaml:"telemetry"`
	Hosts          []HostConfig          `yaml:"hosts"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host     string     `yaml:"host"`
	Port     int        `yaml:"port"`
	Auth     AuthConfig `yaml:"auth"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// ScannerConfig contains scanner settings
type ScannerConfig struct {
	IntervalSeconds int `yaml:"interval_seconds"`
	TimeoutSeconds  int `yaml:"timeout_seconds"`
}

// VulnerabilityConfig contains vulnerability scanner settings
type VulnerabilityConfig struct {
	Enabled                bool   `yaml:"enabled"`
	AutoScanNewImages      bool   `yaml:"auto_scan_new_images"`
	WorkerPoolSize         int    `yaml:"worker_pool_size"`
	ScanTimeoutMinutes     int    `yaml:"scan_timeout_minutes"`
	CacheTTLHours          int    `yaml:"cache_ttl_hours"`
	RescanIntervalHours    int    `yaml:"rescan_interval_hours"`
	CacheDir               string `yaml:"cache_dir"`
	DBUpdateIntervalHours  int    `yaml:"db_update_interval_hours"`
	RetentionDays          int    `yaml:"retention_days"`
	DetailedRetentionDays  int    `yaml:"detailed_retention_days"`
	AlertOnCritical        bool   `yaml:"alert_on_critical"`
	AlertOnHigh            bool   `yaml:"alert_on_high"`
	MaxQueueSize           int    `yaml:"max_queue_size"`
}

// HostConfig contains host configuration
type HostConfig struct {
	Name        string `yaml:"name"`
	Address     string `yaml:"address"`
	Description string `yaml:"description"`
}

// AgentInfo represents agent metadata
type AgentInfo struct {
	Version    string    `json:"version"`
	Hostname   string    `json:"hostname"`
	OS         string    `json:"os"`
	Arch       string    `json:"arch"`
	DockerVersion string `json:"docker_version"`
	StartedAt  time.Time `json:"started_at"`
}

// AgentRequest wraps requests sent to agents
type AgentRequest struct {
	Action    string                 `json:"action"` // scan, start, stop, restart, remove, logs, images, etc.
	Params    map[string]interface{} `json:"params,omitempty"`
}

// AgentResponse wraps responses from agents
type AgentResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// TelemetryConfig contains telemetry settings
type TelemetryConfig struct {
	IntervalHours int                `yaml:"interval_hours"`
	Endpoints     []TelemetryEndpoint `yaml:"endpoints"`
}

// TelemetryEndpoint represents a telemetry submission endpoint
type TelemetryEndpoint struct {
	Name              string     `yaml:"name" json:"name"`
	URL               string     `yaml:"url" json:"url"`
	Enabled           bool       `yaml:"enabled" json:"enabled"`
	APIKey            string     `yaml:"api_key,omitempty" json:"api_key,omitempty"` // Optional API key for authenticated endpoints
	LastSuccess       *time.Time `yaml:"-" json:"last_success,omitempty"`             // Last successful submission
	LastFailure       *time.Time `yaml:"-" json:"last_failure,omitempty"`             // Last failed submission
	LastFailureReason string     `yaml:"-" json:"last_failure_reason,omitempty"`      // Error message from last failure
}

// SystemSettings holds all database-stored configuration
type SystemSettings struct {
	Scanner      ScannerSettings      `json:"scanner"`
	Telemetry    TelemetrySettings    `json:"telemetry"`
	Notification NotificationSettings `json:"notification"`
	UI           UISettings           `json:"ui"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

// ScannerSettings contains runtime scanner configuration
type ScannerSettings struct {
	IntervalSeconds int `json:"interval_seconds" validate:"min=10,max=86400"`
	TimeoutSeconds  int `json:"timeout_seconds" validate:"min=5,max=300"`
}

// TelemetrySettings contains runtime telemetry configuration
type TelemetrySettings struct {
	IntervalHours int `json:"interval_hours" validate:"min=1,max=720"`
}

// NotificationSettings contains runtime notification configuration
type NotificationSettings struct {
	RateLimitMax           int `json:"rate_limit_max" validate:"min=1,max=1000"`
	RateLimitBatchInterval int `json:"rate_limit_batch_interval" validate:"min=60,max=3600"`
	ThresholdDuration      int `json:"threshold_duration" validate:"min=30,max=600"`
	CooldownPeriod         int `json:"cooldown_period" validate:"min=60,max=3600"`
}

// UISettings contains user interface preferences
type UISettings struct {
	CardDesign string `json:"card_design" validate:"oneof=compact material dashboard"`
}

// Validate validates system settings
func (s *SystemSettings) Validate() error {
	if s.Scanner.IntervalSeconds < 10 || s.Scanner.IntervalSeconds > 86400 {
		return fmt.Errorf("scanner interval must be between 10 and 86400 seconds")
	}
	if s.Scanner.TimeoutSeconds < 5 || s.Scanner.TimeoutSeconds > 300 {
		return fmt.Errorf("scanner timeout must be between 5 and 300 seconds")
	}
	if s.Telemetry.IntervalHours < 1 || s.Telemetry.IntervalHours > 720 {
		return fmt.Errorf("telemetry interval must be between 1 and 720 hours")
	}
	if s.Notification.RateLimitMax < 1 || s.Notification.RateLimitMax > 1000 {
		return fmt.Errorf("notification rate limit must be between 1 and 1000")
	}
	if s.Notification.RateLimitBatchInterval < 60 || s.Notification.RateLimitBatchInterval > 3600 {
		return fmt.Errorf("notification batch interval must be between 60 and 3600 seconds")
	}
	if s.Notification.ThresholdDuration < 30 || s.Notification.ThresholdDuration > 600 {
		return fmt.Errorf("notification threshold duration must be between 30 and 600 seconds")
	}
	if s.Notification.CooldownPeriod < 60 || s.Notification.CooldownPeriod > 3600 {
		return fmt.Errorf("notification cooldown period must be between 60 and 3600 seconds")
	}
	// Validate UI settings
	if s.UI.CardDesign != "" && s.UI.CardDesign != "compact" && s.UI.CardDesign != "material" && s.UI.CardDesign != "dashboard" {
		return fmt.Errorf("card design must be one of: compact, material, dashboard")
	}
	return nil
}

// TelemetryReport contains anonymous usage statistics
type TelemetryReport struct {
	InstallationID  string                   `json:"installation_id"`
	Version         string                   `json:"version"`
	Timestamp       time.Time                `json:"timestamp"`
	HostCount       int                      `json:"host_count"`
	AgentCount      int                      `json:"agent_count"`
	TotalContainers int                      `json:"total_containers"`
	ScanInterval    int                      `json:"scan_interval_seconds"`
	ImageStats      []ImageStat              `json:"image_stats"`
	AgentVersions   map[string]int           `json:"agent_versions"` // version -> count
	// Container state breakdown
	ContainersRunning int `json:"containers_running"`
	ContainersStopped int `json:"containers_stopped"`
	ContainersPaused  int `json:"containers_paused"`
	ContainersOther   int `json:"containers_other"`
	// Resource usage aggregates (averages across all running containers)
	AvgCPUPercent     float64 `json:"avg_cpu_percent,omitempty"`
	AvgMemoryBytes    int64   `json:"avg_memory_bytes,omitempty"`
	TotalMemoryLimit  int64   `json:"total_memory_limit,omitempty"`
	// Restart statistics
	AvgRestarts           float64 `json:"avg_restarts,omitempty"`
	HighRestartContainers int     `json:"high_restart_containers,omitempty"` // containers with >10 restarts
	// Image statistics
	TotalImageSize int64 `json:"total_image_size,omitempty"` // total bytes of all images
	UniqueImages   int   `json:"unique_images,omitempty"`
	// System information (optional)
	Timezone string `json:"timezone,omitempty"` // e.g., "America/New_York"
	// Connection and architecture metrics
	ComposeProjectCount  int `json:"compose_project_count,omitempty"`  // number of unique compose projects
	ContainersInCompose  int `json:"containers_in_compose,omitempty"`  // containers with compose_project set
	NetworkCount         int `json:"network_count,omitempty"`          // total networks (including bridge, host)
	CustomNetworkCount   int `json:"custom_network_count,omitempty"`   // user-created networks (excludes bridge, host, none)
	SharedVolumeCount    int `json:"shared_volume_count,omitempty"`    // volumes shared by 2+ containers
	ContainersWithDeps   int `json:"containers_with_deps,omitempty"`   // containers with depends_on configured
	TotalDependencies    int `json:"total_dependencies,omitempty"`     // total dependency edges
	AvgConnectionsPerContainer float64 `json:"avg_connections_per_container,omitempty"` // avg network+volume connections
}

// ImageStat contains statistics for a container image
type ImageStat struct {
	Image     string `json:"image"`
	Count     int    `json:"count"`
	SizeBytes int64  `json:"size_bytes,omitempty"` // total size in bytes
}

// TelemetryStats represents aggregated telemetry data stored in the collector
type TelemetryStats struct {
	ID              int64     `json:"id"`
	InstallationID  string    `json:"installation_id"`
	Version         string    `json:"version"`
	Timestamp       time.Time `json:"timestamp"`
	HostCount       int       `json:"host_count"`
	AgentCount      int       `json:"agent_count"`
	TotalContainers int       `json:"total_containers"`
	ScanInterval    int       `json:"scan_interval_seconds"`
	ImageStatsJSON  string    `json:"-"` // Stored as JSON in DB
	AgentVersionsJSON string  `json:"-"` // Stored as JSON in DB
}

// ContainerLifecycleEvent represents a single lifecycle event for a container
type ContainerLifecycleEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	EventType    string    `json:"event_type"` // "first_seen", "started", "stopped", "restarted", "image_updated", "disappeared"
	OldState     string    `json:"old_state,omitempty"`
	NewState     string    `json:"new_state,omitempty"`
	OldImage     string    `json:"old_image,omitempty"`     // Deprecated: kept for backward compatibility, contains SHA
	NewImage     string    `json:"new_image,omitempty"`     // Deprecated: kept for backward compatibility, contains SHA
	OldImageTag  string    `json:"old_image_tag,omitempty"` // Full image name with tag (e.g., "nginx:1.25.3")
	NewImageTag  string    `json:"new_image_tag,omitempty"` // Full image name with tag (e.g., "nginx:1.25.4")
	OldImageSHA  string    `json:"old_image_sha,omitempty"` // Truncated SHA (12 chars)
	NewImageSHA  string    `json:"new_image_sha,omitempty"` // Truncated SHA (12 chars)
	Description  string    `json:"description"`
	RestartCount int       `json:"restart_count,omitempty"`
}

// ContainerLifecycleSummary represents a summary of a container's lifecycle
type ContainerLifecycleSummary struct {
	ContainerID     string    `json:"container_id"`
	ContainerName   string    `json:"container_name"`
	Image           string    `json:"image"`
	HostID          int64     `json:"host_id"`
	HostName        string    `json:"host_name"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	CurrentState    string    `json:"current_state"`
	StateChanges    int       `json:"state_changes"`
	ImageUpdates    int       `json:"image_updates"`
	RestartEvents   int       `json:"restart_events"`
	IsActive        bool      `json:"is_active"` // seen in most recent scan
	TotalScans      int       `json:"total_scans"`
}

// ContainerStatsPoint represents a single data point for container resource usage
type ContainerStatsPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryUsage   int64     `json:"memory_usage"`   // bytes
	MemoryLimit   int64     `json:"memory_limit"`   // bytes
	MemoryPercent float64   `json:"memory_percent"`
}

// Notification event types
const (
	EventTypeNewImage           = "new_image"
	EventTypeImageUpdateAvailable = "image_update_available"
	EventTypeStateChange        = "state_change"
	EventTypeHighCPU            = "high_cpu"
	EventTypeHighMemory         = "high_memory"
	EventTypeAnomalousBehavior  = "anomalous_behavior"
	EventTypeContainerStarted   = "container_started"
	EventTypeContainerStopped   = "container_stopped"
	EventTypeContainerPaused    = "container_paused"
	EventTypeContainerResumed   = "container_resumed"
)

// Notification channel types
const (
	ChannelTypeWebhook = "webhook"
	ChannelTypeNtfy    = "ntfy"
	ChannelTypeInApp   = "in_app"
)

// NotificationChannel represents a notification delivery channel
type NotificationChannel struct {
	ID        int64                  `json:"id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"` // webhook, ntfy, in_app
	Config    map[string]interface{} `json:"config"`
	Enabled   bool                   `json:"enabled"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// WebhookConfig represents webhook-specific configuration
type WebhookConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// NtfyConfig represents ntfy-specific configuration
type NtfyConfig struct {
	ServerURL string `json:"server_url"` // e.g., https://ntfy.sh or custom server
	Token     string `json:"token,omitempty"`
	Topic     string `json:"topic"`
}

// NotificationRule represents a rule that triggers notifications
type NotificationRule struct {
	ID                       int64     `json:"id"`
	Name                     string    `json:"name"`
	Enabled                  bool      `json:"enabled"`
	EventTypes               []string  `json:"event_types"` // e.g., ["new_image", "state_change"]
	HostID                   *int64    `json:"host_id,omitempty"` // nil = all hosts
	ContainerPattern         string    `json:"container_pattern,omitempty"` // glob pattern
	ImagePattern             string    `json:"image_pattern,omitempty"` // glob pattern
	CPUThreshold             *float64  `json:"cpu_threshold,omitempty"` // nil = no threshold
	MemoryThreshold          *float64  `json:"memory_threshold,omitempty"` // nil = no threshold
	ThresholdDurationSeconds int       `json:"threshold_duration_seconds"`
	CooldownSeconds          int       `json:"cooldown_seconds"`
	ChannelIDs               []int64   `json:"channel_ids"` // channels to send to
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// NotificationLog represents a sent notification
type NotificationLog struct {
	ID            int64                  `json:"id"`
	RuleID        *int64                 `json:"rule_id,omitempty"`
	RuleName      string                 `json:"rule_name,omitempty"`
	ChannelID     *int64                 `json:"channel_id,omitempty"`
	ChannelName   string                 `json:"channel_name,omitempty"`
	EventType     string                 `json:"event_type"`
	ContainerID   string                 `json:"container_id,omitempty"`
	ContainerName string                 `json:"container_name,omitempty"`
	HostID        *int64                 `json:"host_id,omitempty"`
	HostName      string                 `json:"host_name,omitempty"`
	Message       string                 `json:"message"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	SentAt        time.Time              `json:"sent_at"`
	Success       bool                   `json:"success"`
	Error         string                 `json:"error,omitempty"`
	Read          bool                   `json:"read"`
}

// NotificationSilence represents a muted container or host
type NotificationSilence struct {
	ID               int64      `json:"id"`
	HostID           *int64     `json:"host_id,omitempty"`
	ContainerID      string     `json:"container_id,omitempty"`
	ContainerName    string     `json:"container_name,omitempty"`
	HostPattern      string     `json:"host_pattern,omitempty"`
	ContainerPattern string     `json:"container_pattern,omitempty"`
	SilencedUntil    time.Time  `json:"silenced_until"`
	Reason           string     `json:"reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// ContainerBaselineStats represents pre-change baseline for anomaly detection
type ContainerBaselineStats struct {
	ID                int64     `json:"id"`
	ContainerID       string    `json:"container_id"`
	ContainerName     string    `json:"container_name"`
	HostID            int64     `json:"host_id"`
	ImageID           string    `json:"image_id"`
	AvgCPUPercent     float64   `json:"avg_cpu_percent"`
	AvgMemoryPercent  float64   `json:"avg_memory_percent"`
	AvgMemoryUsage    int64     `json:"avg_memory_usage"`
	SampleCount       int       `json:"sample_count"`
	WindowStart       time.Time `json:"window_start"`
	WindowEnd         time.Time `json:"window_end"`
	CreatedAt         time.Time `json:"created_at"`
}

// NotificationThresholdState tracks threshold breach state for cooldowns
type NotificationThresholdState struct {
	ID              int64     `json:"id"`
	ContainerID     string    `json:"container_id"`
	HostID          int64     `json:"host_id"`
	ThresholdType   string    `json:"threshold_type"` // "cpu" or "memory"
	BreachedAt      time.Time `json:"breached_at"`
	LastNotifiedAt  *time.Time `json:"last_notified_at,omitempty"`
	BreachCount     int       `json:"breach_count"`
}

// NotificationEvent represents an event that may trigger notifications
type NotificationEvent struct {
	EventType     string                 `json:"event_type"`
	Timestamp     time.Time              `json:"timestamp"`
	ContainerID   string                 `json:"container_id"`
	ContainerName string                 `json:"container_name"`
	HostID        int64                  `json:"host_id"`
	HostName      string                 `json:"host_name"`
	Image         string                 `json:"image,omitempty"`
	OldState      string                 `json:"old_state,omitempty"`
	NewState      string                 `json:"new_state,omitempty"`
	OldImage      string                 `json:"old_image,omitempty"`
	NewImage      string                 `json:"new_image,omitempty"`
	CPUPercent    float64                `json:"cpu_percent,omitempty"`
	MemoryPercent float64                `json:"memory_percent,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NotificationStatus represents the current notification system status
type NotificationStatus struct {
	UnreadCount        int       `json:"unread_count"`
	TotalChannels      int       `json:"total_channels"`
	EnabledChannels    int       `json:"enabled_channels"`
	TotalRules         int       `json:"total_rules"`
	EnabledRules       int       `json:"enabled_rules"`
	RecentFailures     int       `json:"recent_failures"` // last 24h
	ActiveSilences     int       `json:"active_silences"`
	RateLimitRemaining int       `json:"rate_limit_remaining"`
	RateLimitReset     time.Time `json:"rate_limit_reset"`
}

// ChangesReport represents a summary of environment changes over a time period
type ChangesReport struct {
	Period            ReportPeriod        `json:"period"`
	Summary           ReportSummary       `json:"summary"`
	NewContainers     []ContainerChange   `json:"new_containers"`
	RemovedContainers []ContainerChange   `json:"removed_containers"`
	ImageUpdates      []ImageUpdateChange `json:"image_updates"`
	StateChanges      []StateChange       `json:"state_changes"`
	TopRestarted      []RestartSummary    `json:"top_restarted"`
}

// ReportPeriod represents the time range for a report
type ReportPeriod struct {
	Start         time.Time `json:"start"`
	End           time.Time `json:"end"`
	DurationHours int       `json:"duration_hours"`
}

// ReportSummary contains aggregate statistics for a changes report
type ReportSummary struct {
	TotalHosts        int `json:"total_hosts"`
	TotalContainers   int `json:"total_containers"`
	NewContainers     int `json:"new_containers"`
	RemovedContainers int `json:"removed_containers"`
	ImageUpdates      int `json:"image_updates"`
	StateChanges      int `json:"state_changes"`
	Restarts          int `json:"restarts"`
}

// ContainerChange represents a new or removed container event
type ContainerChange struct {
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	Image         string    `json:"image"`
	HostID        int64     `json:"host_id"`
	HostName      string    `json:"host_name"`
	Timestamp     time.Time `json:"timestamp"` // first_seen or last_seen
	State         string    `json:"state"`
	IsTransient   bool      `json:"is_transient"` // true if container appeared and disappeared in same period
}

// ImageUpdateChange represents an image update event
type ImageUpdateChange struct {
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	HostID        int64     `json:"host_id"`
	HostName      string    `json:"host_name"`
	OldImage      string    `json:"old_image"`
	NewImage      string    `json:"new_image"`
	OldImageID    string    `json:"old_image_id"`
	NewImageID    string    `json:"new_image_id"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// StateChange represents a container state transition event
type StateChange struct {
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	HostID        int64     `json:"host_id"`
	HostName      string    `json:"host_name"`
	OldState      string    `json:"old_state"`
	NewState      string    `json:"new_state"`
	ChangedAt     time.Time `json:"changed_at"`
}

// RestartSummary represents containers with the most restarts
type RestartSummary struct {
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	HostID        int64  `json:"host_id"`
	HostName      string `json:"host_name"`
	RestartCount  int    `json:"restart_count"`
	CurrentState  string `json:"current_state"`
	Image         string `json:"image"`
}

// ImageUpdateInfo contains information about an image update check
type ImageUpdateInfo struct {
	Available     bool      `json:"available"`
	LocalDigest   string    `json:"local_digest"`
	RemoteDigest  string    `json:"remote_digest"`
	RemoteCreated time.Time `json:"remote_created,omitempty"`
	ImageName     string    `json:"image_name"`
	Tag           string    `json:"tag"`
	Error         string    `json:"error,omitempty"`
}

// ContainerRecreateResult contains the result of a container recreation
type ContainerRecreateResult struct {
	Success       bool                   `json:"success"`
	Error         string                 `json:"error,omitempty"`
	OldContainerID string                `json:"old_container_id"`
	NewContainerID string                `json:"new_container_id"`
	OldImageID    string                 `json:"old_image_id"`
	NewImageID    string                 `json:"new_image_id"`
	KeptOldImage  bool                   `json:"kept_old_image"`
	Config        map[string]interface{} `json:"config,omitempty"` // Container config for dry-run preview
}

// ImageUpdateSettings contains runtime image update configuration
type ImageUpdateSettings struct {
	AutoCheckEnabled     bool `json:"auto_check_enabled"`
	CheckIntervalHours   int  `json:"check_interval_hours" validate:"min=1,max=168"`
	OnlyCheckLatestTags  bool `json:"only_check_latest_tags"`
}

// Validate validates image update settings
func (s *ImageUpdateSettings) Validate() error {
	if s.CheckIntervalHours < 1 || s.CheckIntervalHours > 168 {
		return fmt.Errorf("check interval must be between 1 and 168 hours")
	}
	return nil
}
