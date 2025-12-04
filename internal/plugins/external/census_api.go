package external

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/container-census/container-census/internal/models"
	pb "github.com/container-census/container-census/internal/plugins/proto"
	"github.com/container-census/container-census/internal/storage"
)

// CensusAPIServer implements the Census API gRPC service for plugins
type CensusAPIServer struct {
	pb.UnimplementedCensusAPIServer
	db          *storage.DB
	permissions map[string]*PermissionChecker // pluginID -> checker
}

// NewCensusAPIServer creates a new Census API server
func NewCensusAPIServer(db *storage.DB) *CensusAPIServer {
	return &CensusAPIServer{
		db:          db,
		permissions: make(map[string]*PermissionChecker),
	}
}

// RegisterPlugin registers a plugin's permissions for enforcement
func (s *CensusAPIServer) RegisterPlugin(pluginID string, permissions []string) error {
	if err := ValidatePermissions(permissions); err != nil {
		return fmt.Errorf("invalid permissions for plugin %s: %w", pluginID, err)
	}

	s.permissions[pluginID] = NewPermissionChecker(pluginID, permissions)
	log.Printf("[CensusAPI] Registered plugin %s with permissions: %v", pluginID, permissions)
	return nil
}

// UnregisterPlugin removes a plugin's permissions
func (s *CensusAPIServer) UnregisterPlugin(pluginID string) {
	delete(s.permissions, pluginID)
	log.Printf("[CensusAPI] Unregistered plugin %s", pluginID)
}

// checkPermission verifies a plugin has the required permission
func (s *CensusAPIServer) checkPermission(pluginID string, perm Permission) error {
	checker, ok := s.permissions[pluginID]
	if !ok {
		return fmt.Errorf("plugin %s is not registered", pluginID)
	}
	return checker.Check(perm)
}

// GetContainers returns containers
func (s *CensusAPIServer) GetContainers(ctx context.Context, req *pb.GetContainersRequest) (*pb.GetContainersResponse, error) {
	// Check permission
	if err := s.checkPermission(req.PluginId, PermContainersRead); err != nil {
		log.Printf("[CensusAPI] Permission denied: %v", err)
		return nil, err
	}

	log.Printf("[CensusAPI] Plugin %s requesting containers (latest_only=%v, host_id=%d)",
		req.PluginId, req.LatestOnly, req.HostId)

	// Get containers from database
	var containers []models.Container
	var err error

	if req.HostId > 0 {
		containers, err = s.db.GetContainersByHost(req.HostId)
	} else if req.LatestOnly {
		containers, err = s.db.GetLatestContainers()
	} else {
		// For now, default to latest
		containers, err = s.db.GetLatestContainers()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get containers: %w", err)
	}

	// Convert to protobuf format
	pbContainers := make([]*pb.Container, len(containers))
	for i, c := range containers {
		pbContainers[i] = containerToProto(&c)
	}

	return &pb.GetContainersResponse{
		Containers: pbContainers,
	}, nil
}

// GetContainer returns a specific container
func (s *CensusAPIServer) GetContainer(ctx context.Context, req *pb.GetContainerRequest) (*pb.GetContainerResponse, error) {
	// Check permission
	if err := s.checkPermission(req.PluginId, PermContainersRead); err != nil {
		log.Printf("[CensusAPI] Permission denied: %v", err)
		return nil, err
	}

	log.Printf("[CensusAPI] Plugin %s requesting container %s on host %d",
		req.PluginId, req.ContainerId, req.HostId)

	// For simplicity, get all containers and filter
	// In production, this would be a direct query
	containers, err := s.db.GetContainersByHost(req.HostId)
	if err != nil {
		return nil, fmt.Errorf("failed to get containers: %w", err)
	}

	for _, c := range containers {
		if c.ID == req.ContainerId {
			return &pb.GetContainerResponse{
				Container: containerToProto(&c),
				Found:     true,
			}, nil
		}
	}

	return &pb.GetContainerResponse{
		Found: false,
	}, nil
}

// GetHosts returns all hosts
func (s *CensusAPIServer) GetHosts(ctx context.Context, req *pb.GetHostsRequest) (*pb.GetHostsResponse, error) {
	// Check permission
	if err := s.checkPermission(req.PluginId, PermHostsRead); err != nil {
		log.Printf("[CensusAPI] Permission denied: %v", err)
		return nil, err
	}

	log.Printf("[CensusAPI] Plugin %s requesting hosts", req.PluginId)

	hosts, err := s.db.GetHosts()
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts: %w", err)
	}

	pbHosts := make([]*pb.Host, len(hosts))
	for i, h := range hosts {
		pbHosts[i] = hostToProto(&h)
	}

	return &pb.GetHostsResponse{
		Hosts: pbHosts,
	}, nil
}

// GetHost returns a specific host
func (s *CensusAPIServer) GetHost(ctx context.Context, req *pb.GetHostRequest) (*pb.GetHostResponse, error) {
	// Check permission
	if err := s.checkPermission(req.PluginId, PermHostsRead); err != nil {
		log.Printf("[CensusAPI] Permission denied: %v", err)
		return nil, err
	}

	log.Printf("[CensusAPI] Plugin %s requesting host %d", req.PluginId, req.HostId)

	host, err := s.db.GetHost(req.HostId)
	if err != nil {
		return nil, fmt.Errorf("failed to get host: %w", err)
	}

	if host == nil {
		return &pb.GetHostResponse{
			Found: false,
		}, nil
	}

	return &pb.GetHostResponse{
		Host:  hostToProto(host),
		Found: true,
	}, nil
}

// GetPluginData retrieves plugin-specific data
func (s *CensusAPIServer) GetPluginData(ctx context.Context, req *pb.GetPluginDataRequest) (*pb.GetPluginDataResponse, error) {
	// Check permission
	if err := s.checkPermission(req.PluginId, PermStorageRead); err != nil {
		log.Printf("[CensusAPI] Permission denied: %v", err)
		return nil, err
	}

	log.Printf("[CensusAPI] Plugin %s reading data key: %s", req.PluginId, req.Key)

	value, err := s.db.GetPluginData(req.PluginId, req.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin data: %w", err)
	}

	if value == nil {
		return &pb.GetPluginDataResponse{
			Found: false,
		}, nil
	}

	return &pb.GetPluginDataResponse{
		Value: string(value),
		Found: true,
	}, nil
}

// SetPluginData stores plugin-specific data
func (s *CensusAPIServer) SetPluginData(ctx context.Context, req *pb.SetPluginDataRequest) (*pb.SetPluginDataResponse, error) {
	// Check permission
	if err := s.checkPermission(req.PluginId, PermStorageWrite); err != nil {
		log.Printf("[CensusAPI] Permission denied: %v", err)
		return nil, err
	}

	log.Printf("[CensusAPI] Plugin %s writing data key: %s", req.PluginId, req.Key)

	err := s.db.SetPluginData(req.PluginId, req.Key, []byte(req.Value))
	if err != nil {
		return nil, fmt.Errorf("failed to set plugin data: %w", err)
	}

	return &pb.SetPluginDataResponse{
		Success: true,
	}, nil
}

// DeletePluginData deletes plugin-specific data
func (s *CensusAPIServer) DeletePluginData(ctx context.Context, req *pb.DeletePluginDataRequest) (*pb.DeletePluginDataResponse, error) {
	// Check permission
	if err := s.checkPermission(req.PluginId, PermStorageWrite); err != nil {
		log.Printf("[CensusAPI] Permission denied: %v", err)
		return nil, err
	}

	log.Printf("[CensusAPI] Plugin %s deleting data key: %s", req.PluginId, req.Key)

	err := s.db.DeletePluginData(req.PluginId, req.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to delete plugin data: %w", err)
	}

	return &pb.DeletePluginDataResponse{
		Success: true,
	}, nil
}

// Log receives log messages from plugins
func (s *CensusAPIServer) Log(ctx context.Context, req *pb.LogRequest) (*pb.LogResponse, error) {
	// Format log message with plugin ID prefix
	prefix := fmt.Sprintf("[Plugin:%s]", req.PluginId)

	switch req.Level {
	case "debug":
		log.Printf("%s [DEBUG] %s", prefix, req.Message)
	case "info":
		log.Printf("%s [INFO] %s", prefix, req.Message)
	case "warn":
		log.Printf("%s [WARN] %s", prefix, req.Message)
	case "error":
		log.Printf("%s [ERROR] %s", prefix, req.Message)
	default:
		log.Printf("%s %s", prefix, req.Message)
	}

	return &pb.LogResponse{
		Success: true,
	}, nil
}

// SendEvent sends events to other plugins
func (s *CensusAPIServer) SendEvent(ctx context.Context, req *pb.SendEventRequest) (*pb.SendEventResponse, error) {
	log.Printf("[CensusAPI] Plugin %s sending event: %s", req.PluginId, req.EventType)

	// TODO: Implement event bus for inter-plugin communication
	// For now, just log it
	log.Printf("[CensusAPI] Event from %s: type=%s, data=%v", req.PluginId, req.EventType, req.Data)

	return &pb.SendEventResponse{
		Success: true,
	}, nil
}

// Helper functions to convert models to protobuf

func containerToProto(c *models.Container) *pb.Container {
	pbContainer := &pb.Container{
		Id:             c.ID,
		Name:           c.Name,
		Image:          c.Image,
		ImageId:        c.ImageID,
		State:          c.State,
		Status:         c.Status,
		HostId:         c.HostID,
		HostName:       c.HostName,
		Created:        c.Created.Format(time.RFC3339),
		StartedAt:      c.StartedAt.Format(time.RFC3339),
		FinishedAt:     "", // Not in current model
		Networks:       c.Networks,
		Links:          c.Links,
		Labels:         c.Labels,
		Env:            make(map[string]string), // Not in current model
		ComposeProject: c.ComposeProject,
		CpuPercent:     c.CPUPercent,
		MemoryUsage:    c.MemoryUsage,
		MemoryLimit:    c.MemoryLimit,
		MemoryPercent:  c.MemoryPercent,
	}

	// Convert ports
	if c.Ports != nil {
		pbContainer.Ports = make([]*pb.Port, len(c.Ports))
		for i, p := range c.Ports {
			pbContainer.Ports[i] = &pb.Port{
				ContainerPort: int32(p.PrivatePort),
				HostPort:      int32(p.PublicPort),
				Protocol:      p.Type,
				HostIp:        p.IP,
			}
		}
	}

	// Convert volumes
	if c.Volumes != nil {
		pbContainer.Volumes = make([]*pb.Volume, len(c.Volumes))
		for i, v := range c.Volumes {
			mode := "ro"
			if v.RW {
				mode = "rw"
			}
			pbContainer.Volumes[i] = &pb.Volume{
				Type:        v.Type,
				Name:        v.Name,
				Source:      v.Name, // Use name as source
				Destination: v.Destination,
				Mode:        mode,
			}
		}
	}

	return pbContainer
}

func hostToProto(h *models.Host) *pb.Host {
	lastSeen := ""
	if !h.LastSeen.IsZero() {
		lastSeen = h.LastSeen.Format(time.RFC3339)
	}

	return &pb.Host{
		Id:           h.ID,
		Name:         h.Name,
		Address:      h.Address,
		HostType:     h.HostType,
		Description:  h.Description,
		Enabled:      h.Enabled,
		CollectStats: h.CollectStats,
		LastSeen:     lastSeen,
		AgentVersion: h.AgentVersion,
		AgentStatus:  h.AgentStatus,
	}
}
