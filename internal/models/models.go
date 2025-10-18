package models

import "time"

// Host represents a Docker host to scan
type Host struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`      // e.g., "tcp://host:2376", "ssh://user@host", "agent://host:9876"
	Description string    `json:"description"`
	HostType    string    `json:"host_type"`    // unix, tcp, ssh, agent
	AgentToken  string    `json:"agent_token,omitempty"` // API token for agent authentication
	AgentStatus string    `json:"agent_status,omitempty"` // online, offline, unknown
	LastSeen    time.Time `json:"last_seen,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Container represents a Docker container found on a host
type Container struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	ImageID      string            `json:"image_id"`
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
	// Resource usage stats (optional, may be zero if not collected)
	CPUPercent    float64 `json:"cpu_percent,omitempty"`
	MemoryUsage   int64   `json:"memory_usage,omitempty"`   // bytes
	MemoryLimit   int64   `json:"memory_limit,omitempty"`   // bytes
	MemoryPercent float64 `json:"memory_percent,omitempty"`
}

// PortMapping represents a container port mapping
type PortMapping struct {
	PrivatePort int    `json:"private_port"`
	PublicPort  int    `json:"public_port,omitempty"`
	Type        string `json:"type"` // tcp or udp
	IP          string `json:"ip,omitempty"`
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

// Config represents application configuration
type Config struct {
	Database       DatabaseConfig   `yaml:"database"`
	Server         ServerConfig     `yaml:"server"`
	Scanner        ScannerConfig    `yaml:"scanner"`
	Telemetry      TelemetryConfig  `yaml:"telemetry"`
	Hosts          []HostConfig     `yaml:"hosts"`
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
	Enabled       bool               `yaml:"enabled"`
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
