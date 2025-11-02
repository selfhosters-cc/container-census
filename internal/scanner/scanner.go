package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/container-census/container-census/internal/models"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	imagetypes "github.com/docker/docker/api/types/image"
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
	containers, err := dockerClient.ContainerList(ctx, containertypes.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Get image information for size data
	imageMap := make(map[string]int64) // imageID -> size
	images, err := dockerClient.ImageList(ctx, imagetypes.ListOptions{})
	if err == nil {
		for _, img := range images {
			imageMap[img.ID] = img.Size
		}
	}

	// Convert to our model
	result := make([]models.Container, 0, len(containers))
	// Use UTC to ensure consistency across timezones
	now := time.Now().UTC()

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

		// Inspect container for detailed info (restart count, connections, etc.)
		var restartCount int
		var networks []string
		var volumes []models.VolumeMount
		var links []string
		var composeProject string

		containerJSON, err := dockerClient.ContainerInspect(ctx, c.ID)
		if err == nil {
			restartCount = containerJSON.RestartCount

			// Extract network connections
			if containerJSON.NetworkSettings != nil && containerJSON.NetworkSettings.Networks != nil {
				for networkName := range containerJSON.NetworkSettings.Networks {
					networks = append(networks, networkName)
				}
			}

			// Extract volume mounts
			if containerJSON.Mounts != nil {
				for _, mount := range containerJSON.Mounts {
					volumeMount := models.VolumeMount{
						Name:        mount.Name,
						Destination: mount.Destination,
						Type:        string(mount.Type),
						RW:          mount.RW,
					}
					// For bind mounts, use source path as name
					if mount.Type == "bind" {
						volumeMount.Name = mount.Source
					}
					volumes = append(volumes, volumeMount)
				}
			}

			// Extract legacy links
			if containerJSON.HostConfig != nil && containerJSON.HostConfig.Links != nil {
				links = containerJSON.HostConfig.Links
			}

			// Extract Docker Compose project name from labels
			if containerJSON.Config != nil && containerJSON.Config.Labels != nil {
				if project, ok := containerJSON.Config.Labels["com.docker.compose.project"]; ok {
					composeProject = project
				}
			}
		}

		container := models.Container{
			ID:             c.ID,
			Name:           name,
			Image:          c.Image,
			ImageID:        c.ImageID,
			ImageSize:      imageSize,
			State:          c.State,
			Status:         c.Status,
			RestartCount:   restartCount,
			Ports:          ports,
			Labels:         c.Labels,
			Created:        time.Unix(c.Created, 0),
			HostID:         host.ID,
			HostName:       host.Name,
			ScannedAt:      now,
			Networks:       networks,
			Volumes:        volumes,
			Links:          links,
			ComposeProject: composeProject,
		}

		result = append(result, container)
	}

	// Collect stats concurrently for all running containers if enabled for this host
	if host.CollectStats {
		var wg sync.WaitGroup
		var mu sync.Mutex

		for i := range result {
			if result[i].State != "running" {
				continue
			}

			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				containerID := result[idx].ID
				containerName := result[idx].Name

				// Use streaming stats to get two samples
				statsStream, err := dockerClient.ContainerStats(ctx, containerID, true)
				if err != nil {
					log.Printf("Failed to collect stats for container %s on host %s: %v", containerName, host.Name, err)
					return
				}
				defer statsStream.Body.Close()

				// Read first sample (baseline)
				var baseline containertypes.StatsResponse
				decoder := json.NewDecoder(statsStream.Body)
				if err := decoder.Decode(&baseline); err != nil {
					log.Printf("Failed to decode first sample for container %s on host %s: %v", containerName, host.Name, err)
					return
				}

				// Read second sample (current)
				var current containertypes.StatsResponse
				if err := decoder.Decode(&current); err != nil {
					log.Printf("Failed to decode second sample for container %s on host %s: %v", containerName, host.Name, err)
					return
				}

				// Calculate CPU percentage using delta between the two samples
				cpuDelta := float64(current.CPUStats.CPUUsage.TotalUsage - baseline.CPUStats.CPUUsage.TotalUsage)
				systemDelta := float64(current.CPUStats.SystemUsage - baseline.CPUStats.SystemUsage)

				// Get number of CPUs - try multiple sources
				numCPUs := uint64(len(current.CPUStats.CPUUsage.PercpuUsage))
				if numCPUs == 0 && current.CPUStats.OnlineCPUs > 0 {
					numCPUs = uint64(current.CPUStats.OnlineCPUs)
				}
				if numCPUs == 0 {
					// Fallback: assume at least 1 CPU for calculation
					numCPUs = 1
				}

				// Debug logging for CPU calculation
				log.Printf("DEBUG %s: cpuDelta=%.0f, systemDelta=%.0f, numCPUs=%d, OnlineCPUs=%d, PercpuLen=%d",
					containerName, cpuDelta, systemDelta, numCPUs,
					current.CPUStats.OnlineCPUs, len(current.CPUStats.CPUUsage.PercpuUsage))

				var cpuPercent float64
				if systemDelta > 0 && cpuDelta > 0 {
					cpuPercent = (cpuDelta / systemDelta) * float64(numCPUs) * 100.0
				}

				// Memory stats (from the latest sample)
				memoryUsage := int64(current.MemoryStats.Usage)
				memoryLimit := int64(current.MemoryStats.Limit)
				var memoryPercent float64
				if current.MemoryStats.Limit > 0 {
					memoryPercent = float64(current.MemoryStats.Usage) / float64(current.MemoryStats.Limit) * 100.0
				}

				// Debug logging
				log.Printf("Stats collected for %s on %s: CPU=%.2f%%, Memory=%dMB/%dMB (%.1f%%)",
					containerName, host.Name, cpuPercent, memoryUsage/1024/1024, memoryLimit/1024/1024, memoryPercent)

				// Update the container in the result slice (thread-safe)
				mu.Lock()
				result[idx].CPUPercent = cpuPercent
				result[idx].MemoryUsage = memoryUsage
				result[idx].MemoryLimit = memoryLimit
				result[idx].MemoryPercent = memoryPercent
				mu.Unlock()
			}(i)
		}

		wg.Wait()
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

// VerifyAgentAuth checks if agent auth token is valid by testing an authenticated endpoint
func (s *Scanner) VerifyAgentAuth(ctx context.Context, host models.Host) error {
	// Test the /api/containers endpoint which requires authentication
	resp, err := s.agentRequest(ctx, host, "GET", "/api/containers", nil)
	if err != nil {
		return fmt.Errorf("failed to connect to agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed (401): %s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
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

	return dockerClient.ContainerStart(ctx, containerID, containertypes.StartOptions{})
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

	return dockerClient.ContainerRemove(ctx, containerID, containertypes.RemoveOptions{
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

	options := containertypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
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
func (s *Scanner) ListImages(ctx context.Context, host models.Host) ([]imagetypes.Summary, error) {
	if isAgentHost(host.Address) {
		return s.listAgentImages(ctx, host)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	images, err := dockerClient.ImageList(ctx, imagetypes.ListOptions{All: true})
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

	_, err = dockerClient.ImageRemove(ctx, imageID, imagetypes.RemoveOptions{
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
