package scanner

import (
	"context"
	"fmt"
	"io"
	"log"
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

		result = append(result, models.Container{
			ID:        c.ID,
			Name:      name,
			Image:     c.Image,
			ImageID:   c.ImageID,
			State:     c.State,
			Status:    c.Status,
			Ports:     ports,
			Labels:    c.Labels,
			Created:   time.Unix(c.Created, 0),
			HostID:    host.ID,
			HostName:  host.Name,
			ScannedAt: now,
		})
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

	dockerClient, err := s.createClient(address)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	_, err = dockerClient.Ping(ctx)
	return err
}

// Container Management Operations

// StartContainer starts a container on a specific host
func (s *Scanner) StartContainer(ctx context.Context, host models.Host, containerID string) error {
	dockerClient, err := s.createClient(host.Address)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	return dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

// StopContainer stops a container on a specific host
func (s *Scanner) StopContainer(ctx context.Context, host models.Host, containerID string, timeout int) error {
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
