package scanner

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Scanner handles Docker host scanning
type Scanner struct {
	timeout time.Duration
}

// New creates a new Scanner
func New(timeoutSeconds int) *Scanner {
	return &Scanner{
		timeout: time.Duration(timeoutSeconds) * time.Second,
	}
}

// ScanHost scans a single Docker host and returns containers
func (s *Scanner) ScanHost(ctx context.Context, host models.Host) ([]models.Container, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Check if this is an agent host
	if isAgentHost(host.Address) {
		return s.scanAgentHost(ctx, host)
	}

	// Create Docker client
	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	// List containers (including stopped ones)
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Get image information for size data
	imageMap := make(map[string]int64) // imageID -> size
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{})
	if err == nil {
		for _, img := range images {
			imageMap[img.ID] = img.Size
		}
	}

	// Convert to our model
	result := make([]models.Container, 0, len(containers))
	now := time.Now()

	for _, c := range containers {
		// Parse port mappings
		ports := make([]models.PortMapping, 0)
		for _, port := range c.Ports {
			ports = append(ports, models.PortMapping{
				PrivatePort: int(port.PrivatePort),
				PublicPort:  int(port.PublicPort),
				Type:        port.Type,
				IP:          port.IP,
			})
		}

		// Get container name (remove leading slash)
		name := strings.TrimPrefix(c.Names[0], "/")
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		// Get image size
		imageSize := imageMap[c.ImageID]

		// Inspect container for detailed info (restart count, etc.)
		var restartCount int
		containerJSON, err := dockerClient.ContainerInspect(ctx, c.ID)
		if err == nil {
			restartCount = containerJSON.RestartCount
		}

		container := models.Container{
			ID:           c.ID,
			Name:         name,
			Image:        c.Image,
			ImageID:      c.ImageID,
			ImageSize:    imageSize,
			State:        c.State,
			Status:       c.Status,
			RestartCount: restartCount,
			Ports:        ports,
			Labels:       c.Labels,
			Created:      time.Unix(c.Created, 0),
			HostID:       host.ID,
			HostName:     host.Name,
			ScannedAt:    now,
		}

		// Optionally collect resource stats for running containers
		// This is commented out by default as it adds overhead
		// Uncomment if you want to collect resource usage
		/*
		if c.State == "running" {
			stats, err := dockerClient.ContainerStats(ctx, c.ID, false)
			if err == nil {
				defer stats.Body.Close()
				var v types.StatsJSON
				if err := json.NewDecoder(stats.Body).Decode(&v); err == nil {
					// Calculate CPU percentage
					cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
					systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
					if systemDelta > 0 && cpuDelta > 0 {
						container.CPUPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
					}

					// Memory stats
					container.MemoryUsage = int64(v.MemoryStats.Usage)
					container.MemoryLimit = int64(v.MemoryStats.Limit)
					if v.MemoryStats.Limit > 0 {
						container.MemoryPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
					}
				}
			}
		}
		*/

		result = append(result, container)
	}

	return result, nil
}

// ScanAllHosts scans all enabled hosts
func (s *Scanner) ScanAllHosts(ctx context.Context, hosts []models.Host) map[int64]models.ScanResult {
	results := make(map[int64]models.ScanResult)

	for _, host := range hosts {
		if !host.Enabled {
			continue
		}

		result := models.ScanResult{
			HostID:    host.ID,
			HostName:  host.Name,
			StartedAt: time.Now(),
			Success:   false,
		}

		containers, err := s.ScanHost(ctx, host)
		if err != nil {
			result.Error = err.Error()
			log.Printf("Error scanning host %s: %v", host.Name, err)
		} else {
			result.Success = true
			result.ContainersFound = len(containers)
		}

		result.CompletedAt = time.Now()
		results[host.ID] = result
	}

	return results
}

// createClient creates a Docker client based on the address type
func (s *Scanner) createClient(address string) (*client.Client, error) {
	// Support different connection types
	switch {
	case strings.HasPrefix(address, "tcp://"):
		// TCP connection (Docker API over network)
		return client.NewClientWithOpts(
			client.WithHost(address),
			client.WithAPIVersionNegotiation(),
		)
	case strings.HasPrefix(address, "unix://"):
		// Unix socket
		return client.NewClientWithOpts(
			client.WithHost(address),
			client.WithAPIVersionNegotiation(),
		)
	case address == "" || address == "local":
		// Local Docker daemon
		return client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
	case strings.HasPrefix(address, "ssh://"):
		// SSH connection - using Docker's SSH support
		return client.NewClientWithOpts(
			client.WithHost(address),
			client.WithAPIVersionNegotiation(),
		)
	default:
		return nil, fmt.Errorf("unsupported address format: %s", address)
	}
}

// VerifyConnection tests if a host is reachable
func (s *Scanner) VerifyConnection(ctx context.Context, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if this is an agent host
	if isAgentHost(address) {
		return s.verifyAgentConnection(ctx, address)
	}

	dockerClient, err := s.createClient(address)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	_, err = dockerClient.Ping(ctx)
	return err
}

// verifyAgentConnection checks if an agent is reachable via HTTP
func (s *Scanner) verifyAgentConnection(ctx context.Context, address string) error {
	agentURL := normalizeAgentURL(address) + "/health"

	req, err := http.NewRequestWithContext(ctx, "GET", agentURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// Container Management Operations

// StartContainer starts a container on a specific host
func (s *Scanner) StartContainer(ctx context.Context, host models.Host, containerID string) error {
	if isAgentHost(host.Address) {
		resp, err := s.agentRequest(ctx, host, "POST", "/api/containers/"+containerID+"/start", nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	return dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

// StopContainer stops a container on a specific host
func (s *Scanner) StopContainer(ctx context.Context, host models.Host, containerID string, timeout int) error {
	if isAgentHost(host.Address) {
		return s.stopAgentContainer(ctx, host, containerID, timeout)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	stopOptions := containertypes.StopOptions{
		Timeout: &timeout,
	}
	return dockerClient.ContainerStop(ctx, containerID, stopOptions)
}

// RestartContainer restarts a container on a specific host
func (s *Scanner) RestartContainer(ctx context.Context, host models.Host, containerID string, timeout int) error {
	if isAgentHost(host.Address) {
		return s.restartAgentContainer(ctx, host, containerID, timeout)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	stopOptions := containertypes.StopOptions{
		Timeout: &timeout,
	}
	return dockerClient.ContainerRestart(ctx, containerID, stopOptions)
}

// RemoveContainer removes a container on a specific host
func (s *Scanner) RemoveContainer(ctx context.Context, host models.Host, containerID string, force bool) error {
	if isAgentHost(host.Address) {
		return s.removeAgentContainer(ctx, host, containerID, force)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	return dockerClient.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: force,
	})
}

// GetContainerLogs retrieves logs from a container
func (s *Scanner) GetContainerLogs(ctx context.Context, host models.Host, containerID string, tail string) (string, error) {
	if isAgentHost(host.Address) {
		return s.getAgentContainerLogs(ctx, host, containerID, tail)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return "", fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	}

	logs, err := dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer logs.Close()

	// Read logs
	buf, err := io.ReadAll(logs)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(buf), nil
}

// Image Management Operations

// ListImages lists all images on a specific host
func (s *Scanner) ListImages(ctx context.Context, host models.Host) ([]types.ImageSummary, error) {
	if isAgentHost(host.Address) {
		return s.listAgentImages(ctx, host)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	return images, nil
}

// RemoveImage removes an image from a specific host
func (s *Scanner) RemoveImage(ctx context.Context, host models.Host, imageID string, force bool) error {
	if isAgentHost(host.Address) {
		return s.removeAgentImage(ctx, host, imageID, force)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	_, err = dockerClient.ImageRemove(ctx, imageID, types.ImageRemoveOptions{
		Force: force,
	})
	return err
}

// PruneImages removes unused images from a specific host
func (s *Scanner) PruneImages(ctx context.Context, host models.Host) (uint64, error) {
	if isAgentHost(host.Address) {
		return s.pruneAgentImages(ctx, host)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return 0, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	report, err := dockerClient.ImagesPrune(ctx, filters.Args{})
	if err != nil {
		return 0, fmt.Errorf("failed to prune images: %w", err)
	}

	return report.SpaceReclaimed, nil
}

// GetAgentInfo retrieves agent information for telemetry
func (s *Scanner) GetAgentInfo(ctx context.Context, host models.Host) (*models.AgentInfo, error) {
	if !isAgentHost(host.Address) {
		return nil, fmt.Errorf("host is not an agent")
	}

	return s.getAgentInfo(ctx, host)
}
