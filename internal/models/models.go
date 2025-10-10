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
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	ImageID     string            `json:"image_id"`
	State       string            `json:"state"`       // running, exited, etc.
	Status      string            `json:"status"`      // detailed status
	Ports       []PortMapping     `json:"ports"`
	Labels      map[string]string `json:"labels"`
	Created     time.Time         `json:"created"`
	HostID      int64             `json:"host_id"`
	HostName    string            `json:"host_name"`
	ScannedAt   time.Time         `json:"scanned_at"`
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
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
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
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key,omitempty"`
}

// TelemetryReport contains anonymous usage statistics
type TelemetryReport struct {
	InstallationID string                   `json:"installation_id"`
	Version        string                   `json:"version"`
	Timestamp      time.Time                `json:"timestamp"`
	HostCount      int                      `json:"host_count"`
	AgentCount     int                      `json:"agent_count"`
	TotalContainers int                     `json:"total_containers"`
	ScanInterval   int                      `json:"scan_interval_seconds"`
	ImageStats     []ImageStat              `json:"image_stats"`
	AgentVersions  map[string]int           `json:"agent_versions"` // version -> count
}

// ImageStat contains statistics for a container image
type ImageStat struct {
	Image string `json:"image"`
	Count int    `json:"count"`
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
