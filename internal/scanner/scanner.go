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

	// Get image information for size data and version labels
	imageMap := make(map[string]int64)     // imageID -> size
	imageTagsMap := make(map[string][]string) // imageID -> all tags (including version from labels)
	images, err := dockerClient.ImageList(ctx, imagetypes.ListOptions{})
	if err == nil {
		for _, img := range images {
			imageMap[img.ID] = img.Size

			// Start with RepoTags
			tags := make([]string, 0)
			if len(img.RepoTags) > 0 {
				tags = append(tags, img.RepoTags...)
			}

			// Try to extract version from image labels
			if version, ok := img.Labels["org.opencontainers.image.version"]; ok && version != "" {
				// Add version as a tag if it's not already present
				versionTag := version
				// Only add if it's not already in the tags
				found := false
				for _, tag := range tags {
					if tag == versionTag || strings.HasSuffix(tag, ":"+versionTag) {
						found = true
						break
					}
				}
				if !found {
					tags = append(tags, versionTag)
				}
			}

			if len(tags) > 0 {
				imageTagsMap[img.ID] = tags
			}
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

		// Get image size and tags
		imageSize := imageMap[c.ImageID]
		imageTags := imageTagsMap[c.ImageID]

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
			ImageTags:      imageTags,
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
		// Check if the error is due to unsupported logging driver
		if strings.Contains(err.Error(), "configured logging driver does not support reading") {
			return "Logs unavailable: This container uses a logging driver that doesn't support reading logs (e.g., 'none', 'syslog', 'journald'). To view logs, reconfigure the container to use 'json-file' or 'local' logging driver.", nil
		}
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

// Image Update Operations

// CheckImageUpdate checks if a newer version of a container's image is available
func (s *Scanner) CheckImageUpdate(ctx context.Context, host models.Host, imageName, localDigest string) (*models.ImageUpdateInfo, error) {
	// This is handled by the registry client, not Docker API
	// The scanner just provides a convenient wrapper that could be extended
	// to handle agent-specific logic if needed in the future
	return nil, fmt.Errorf("use registry client directly for update checks")
}

// PullImage pulls an image on a specific host
func (s *Scanner) PullImage(ctx context.Context, host models.Host, imageName string) error {
	if isAgentHost(host.Address) {
		return s.pullAgentImage(ctx, host, imageName)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	// Pull the image
	reader, err := dockerClient.ImagePull(ctx, imageName, imagetypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Read the output to ensure the pull completes
	// We don't stream progress here, just wait for completion
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to complete image pull: %w", err)
	}

	return nil
}

// RecreateContainer recreates a container with a new image while preserving configuration
func (s *Scanner) RecreateContainer(ctx context.Context, host models.Host, containerID string, dryRun bool) (*models.ContainerRecreateResult, error) {
	if isAgentHost(host.Address) {
		return s.recreateAgentContainer(ctx, host, containerID, dryRun)
	}

	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	// Inspect the container to get its configuration
	containerJSON, err := dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	oldImageID := containerJSON.Image
	imageName := containerJSON.Config.Image

	// Build the container config for preview/recreation
	config := map[string]interface{}{
		"name":          containerJSON.Name,
		"image":         imageName,
		"env":           containerJSON.Config.Env,
		"cmd":           containerJSON.Config.Cmd,
		"entrypoint":    containerJSON.Config.Entrypoint,
		"working_dir":   containerJSON.Config.WorkingDir,
		"user":          containerJSON.Config.User,
		"exposed_ports": containerJSON.Config.ExposedPorts,
		"labels":        containerJSON.Config.Labels,
		"volumes":       containerJSON.Config.Volumes,
		"host_config": map[string]interface{}{
			"binds":           containerJSON.HostConfig.Binds,
			"port_bindings":   containerJSON.HostConfig.PortBindings,
			"restart_policy":  containerJSON.HostConfig.RestartPolicy,
			"network_mode":    containerJSON.HostConfig.NetworkMode,
			"privileged":      containerJSON.HostConfig.Privileged,
			"cap_add":         containerJSON.HostConfig.CapAdd,
			"cap_drop":        containerJSON.HostConfig.CapDrop,
			"dns":             containerJSON.HostConfig.DNS,
			"extra_hosts":     containerJSON.HostConfig.ExtraHosts,
			"volume_driver":   containerJSON.HostConfig.VolumeDriver,
			"volumes_from":    containerJSON.HostConfig.VolumesFrom,
		},
		"network_settings": map[string]interface{}{
			"networks": containerJSON.NetworkSettings.Networks,
		},
	}

	// If dry-run, return the config without executing
	if dryRun {
		return &models.ContainerRecreateResult{
			Success:        true,
			OldContainerID: containerID,
			OldImageID:     oldImageID,
			Config:         config,
		}, nil
	}

	// Stop the container
	timeout := 10
	stopOptions := containertypes.StopOptions{
		Timeout: &timeout,
	}
	if err := dockerClient.ContainerStop(ctx, containerID, stopOptions); err != nil {
		return nil, fmt.Errorf("failed to stop container: %w", err)
	}

	// Remove the old container (but keep volumes)
	if err := dockerClient.ContainerRemove(ctx, containerID, containertypes.RemoveOptions{
		RemoveVolumes: false,
		Force:         false,
	}); err != nil {
		return nil, fmt.Errorf("failed to remove old container: %w", err)
	}

	// Create new container with the same configuration
	containerName := strings.TrimPrefix(containerJSON.Name, "/")

	createResp, err := dockerClient.ContainerCreate(
		ctx,
		containerJSON.Config,
		containerJSON.HostConfig,
		nil, // NetworkingConfig will be set via network connect
		nil, // Platform
		containerName,
	)
	if err != nil {
		// Try to restart the old container if creation fails
		// (but it's already removed, so this will likely fail too)
		return nil, fmt.Errorf("failed to create new container: %w", err)
	}

	newContainerID := createResp.ID

	// Connect to networks (excluding the default network which is handled by NetworkMode)
	for networkName, networkConfig := range containerJSON.NetworkSettings.Networks {
		// Skip the default bridge network as it's handled by NetworkMode
		if networkName == "bridge" && containerJSON.HostConfig.NetworkMode == "bridge" {
			continue
		}

		err = dockerClient.NetworkConnect(ctx, networkName, newContainerID, networkConfig)
		if err != nil {
			log.Printf("Warning: failed to connect to network %s: %v", networkName, err)
		}
	}

	// Start the new container
	if err := dockerClient.ContainerStart(ctx, newContainerID, containertypes.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start new container: %w", err)
	}

	// Get the new image ID
	newContainerJSON, err := dockerClient.ContainerInspect(ctx, newContainerID)
	if err != nil {
		log.Printf("Warning: failed to inspect new container: %v", err)
	}
	newImageID := newContainerJSON.Image

	return &models.ContainerRecreateResult{
		Success:        true,
		OldContainerID: containerID,
		NewContainerID: newContainerID,
		OldImageID:     oldImageID,
		NewImageID:     newImageID,
		KeptOldImage:   true, // We don't remove the old image
		Config:         config,
	}, nil
}
