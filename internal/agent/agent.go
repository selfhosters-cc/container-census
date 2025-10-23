package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	"github.com/gorilla/mux"
)

// Info represents agent information
type Info struct {
	Version       string    `json:"version"`
	Hostname      string    `json:"hostname"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	DockerVersion string    `json:"docker_version"`
	StartedAt     time.Time `json:"started_at"`
}

// Agent handles Docker operations on a single host
type Agent struct {
	dockerClient *client.Client
	apiToken     string
	info         Info
	router       *mux.Router
}

// New creates a new agent
func New(dockerHost string, apiToken string, info Info) (*Agent, error) {
	// Create Docker client
	dockerClient, err := createDockerClient(dockerHost)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Get Docker version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverVersion, err := dockerClient.ServerVersion(ctx)
	if err != nil {
		log.Printf("Warning: Could not get Docker version: %v", err)
		info.DockerVersion = "unknown"
	} else {
		info.DockerVersion = serverVersion.Version
	}

	a := &Agent{
		dockerClient: dockerClient,
		apiToken:     apiToken,
		info:         info,
		router:       mux.NewRouter(),
	}

	a.setupRoutes()
	return a, nil
}

// setupRoutes configures API routes
func (a *Agent) setupRoutes() {
	// Public routes
	a.router.HandleFunc("/health", a.handleHealth).Methods("GET")
	a.router.HandleFunc("/info", a.handleInfo).Methods("GET")

	// Protected routes (require authentication)
	api := a.router.PathPrefix("/api").Subrouter()
	api.Use(a.authMiddleware)

	api.HandleFunc("/containers", a.handleListContainers).Methods("GET")
	api.HandleFunc("/containers/{id}/start", a.handleStartContainer).Methods("POST")
	api.HandleFunc("/containers/{id}/stop", a.handleStopContainer).Methods("POST")
	api.HandleFunc("/containers/{id}/restart", a.handleRestartContainer).Methods("POST")
	api.HandleFunc("/containers/{id}/remove", a.handleRemoveContainer).Methods("DELETE")
	api.HandleFunc("/containers/{id}/logs", a.handleGetLogs).Methods("GET")

	api.HandleFunc("/images", a.handleListImages).Methods("GET")
	api.HandleFunc("/images/{id}/remove", a.handleRemoveImage).Methods("DELETE")
	api.HandleFunc("/images/prune", a.handlePruneImages).Methods("POST")

	// Telemetry endpoint
	api.HandleFunc("/telemetry", a.handleGetTelemetry).Methods("GET")
}

// Router returns the configured router
func (a *Agent) Router() *mux.Router {
	return a.router
}

// authMiddleware validates API token
func (a *Agent) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-API-Token")
		if token == "" {
			// Also check Authorization header
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if token != a.apiToken {
			respondError(w, http.StatusUnauthorized, "Invalid or missing API token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Health check
func (a *Agent) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, err := a.dockerClient.Ping(ctx)

	health := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}

	if err != nil {
		health["status"] = "unhealthy"
		health["docker_error"] = err.Error()
		respondJSON(w, http.StatusServiceUnavailable, health)
		return
	}

	respondJSON(w, http.StatusOK, health)
}

// Agent info
func (a *Agent) handleInfo(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, a.info)
}

// Container operations
func (a *Agent) handleListContainers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	containers, err := a.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list containers: "+err.Error())
		return
	}

	// Convert to our model
	result := make([]models.Container, 0, len(containers))
	now := time.Now()

	for _, c := range containers {
		ports := make([]models.PortMapping, 0)
		for _, port := range c.Ports {
			ports = append(ports, models.PortMapping{
				PrivatePort: int(port.PrivatePort),
				PublicPort:  int(port.PublicPort),
				Type:        port.Type,
				IP:          port.IP,
			})
		}

		name := strings.TrimPrefix(c.Names[0], "/")
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		// Inspect container for detailed connection info
		var restartCount int
		var networks []string
		var volumes []models.VolumeMount
		var links []string
		var composeProject string

		containerJSON, err := a.dockerClient.ContainerInspect(ctx, c.ID)
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

		result = append(result, models.Container{
			ID:             c.ID,
			Name:           name,
			Image:          c.Image,
			ImageID:        c.ImageID,
			State:          c.State,
			Status:         c.Status,
			RestartCount:   restartCount,
			Ports:          ports,
			Labels:         c.Labels,
			Created:        time.Unix(c.Created, 0),
			ScannedAt:      now,
			Networks:       networks,
			Volumes:        volumes,
			Links:          links,
			ComposeProject: composeProject,
		})
	}

	respondJSON(w, http.StatusOK, result)
}

func (a *Agent) handleStartContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	ctx := r.Context()
	if err := a.dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to start container: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Container started"})
}

func (a *Agent) handleStopContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	timeout := 10
	if t := r.URL.Query().Get("timeout"); t != "" {
		fmt.Sscanf(t, "%d", &timeout)
	}

	ctx := r.Context()
	stopOptions := containertypes.StopOptions{
		Timeout: &timeout,
	}

	if err := a.dockerClient.ContainerStop(ctx, containerID, stopOptions); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to stop container: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Container stopped"})
}

func (a *Agent) handleRestartContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	timeout := 10
	if t := r.URL.Query().Get("timeout"); t != "" {
		fmt.Sscanf(t, "%d", &timeout)
	}

	ctx := r.Context()
	stopOptions := containertypes.StopOptions{
		Timeout: &timeout,
	}

	if err := a.dockerClient.ContainerRestart(ctx, containerID, stopOptions); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to restart container: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Container restarted"})
}

func (a *Agent) handleRemoveContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	force := r.URL.Query().Get("force") == "true"

	ctx := r.Context()
	if err := a.dockerClient.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: force,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to remove container: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Container removed"})
}

func (a *Agent) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerID := vars["id"]

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}

	ctx := r.Context()
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	}

	logs, err := a.dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get logs: "+err.Error())
		return
	}
	defer logs.Close()

	buf, err := io.ReadAll(logs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to read logs: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"logs": string(buf)})
}

// Image operations
func (a *Agent) handleListImages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	images, err := a.dockerClient.ImageList(ctx, types.ImageListOptions{All: true})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list images: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, images)
}

func (a *Agent) handleRemoveImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	imageID := vars["id"]

	force := r.URL.Query().Get("force") == "true"

	ctx := r.Context()
	_, err := a.dockerClient.ImageRemove(ctx, imageID, types.ImageRemoveOptions{
		Force: force,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to remove image: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Image removed"})
}

func (a *Agent) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	report, err := a.dockerClient.ImagesPrune(ctx, filters.Args{})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to prune images: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":         "Images pruned",
		"space_reclaimed": report.SpaceReclaimed,
	})
}

// Telemetry endpoint - returns agent stats for server aggregation
func (a *Agent) handleGetTelemetry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get container list
	containers, err := a.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list containers: "+err.Error())
		return
	}

	// Aggregate image statistics (anonymized)
	imageMap := make(map[string]int)
	for _, container := range containers {
		imageMap[container.Image]++
	}

	// Convert to slice
	imageStats := make([]map[string]interface{}, 0, len(imageMap))
	for image, count := range imageMap {
		imageStats = append(imageStats, map[string]interface{}{
			"image": image,
			"count": count,
		})
	}

	// Return telemetry data
	telemetry := map[string]interface{}{
		"version":          a.info.Version,
		"container_count":  len(containers),
		"image_stats":      imageStats,
		"docker_version":   a.info.DockerVersion,
		"os":               a.info.OS,
		"arch":             a.info.Arch,
	}

	respondJSON(w, http.StatusOK, telemetry)
}

// RegisterWithServer attempts to register this agent with a central server
func (a *Agent) RegisterWithServer(serverURL string) error {
	// This is a placeholder for future auto-registration feature
	// For now, agents must be manually added through the UI
	return fmt.Errorf("auto-registration not yet implemented - please add agent manually through the UI")
}

// Helper functions
func createDockerClient(dockerHost string) (*client.Client, error) {
	if dockerHost == "" || dockerHost == "local" {
		return client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
	}

	return client.NewClientWithOpts(
		client.WithHost(dockerHost),
		client.WithAPIVersionNegotiation(),
	)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// GenerateToken generates a random API token
func GenerateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based token
		return fmt.Sprintf("token_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
