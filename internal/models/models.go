package models

import "time"

// Host represents a Docker host to scan
type Host struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`      // e.g., "tcp://host:2376" or "ssh://user@host"
	Description string    `json:"description"`
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
	Database       DatabaseConfig `yaml:"database"`
	Server         ServerConfig   `yaml:"server"`
	Scanner        ScannerConfig  `yaml:"scanner"`
	Hosts          []HostConfig   `yaml:"hosts"`
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
