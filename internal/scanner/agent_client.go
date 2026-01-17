package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/selfhosters-cc/container-census/internal/models"
	imagetypes "github.com/docker/docker/api/types/image"
)

// AgentClient handles communication with remote agents
type AgentClient struct {
	timeout  int
}

func isAgentHost(address string) bool {
	return strings.HasPrefix(address, "agent://") ||
	       strings.HasPrefix(address, "http://") ||
	       strings.HasPrefix(address, "https://")
}

func normalizeAgentURL(address string) string {
	address = strings.TrimPrefix(address, "agent://")
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	return strings.TrimSuffix(address, "/")
}

func (s *Scanner) agentRequest(ctx context.Context, host models.Host, method, path string, body interface{}) (*http.Response, error) {
	return s.agentRequestWithTimeout(ctx, host, method, path, body, s.timeout)
}

// agentRequestWithTimeout makes a request to an agent with a custom timeout
// Use this for long-running operations like image pulls
func (s *Scanner) agentRequestWithTimeout(ctx context.Context, host models.Host, method, path string, body interface{}, timeout time.Duration) (*http.Response, error) {
	agentURL := normalizeAgentURL(host.Address) + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, agentURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Token", host.AgentToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: timeout}
	return client.Do(req)
}

func (s *Scanner) scanAgentHost(ctx context.Context, host models.Host) ([]models.Container, error) {
	// Add stats query parameter if enabled for this host
	path := "/api/containers"
	if host.CollectStats {
		path += "?stats=true"
		// Stats collection can take longer on hosts with many containers
		// Override the default timeout for this request
		statsCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		ctx = statsCtx
	}

	resp, err := s.agentRequest(ctx, host, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("agent returned status %d: %s (API token mismatch - please verify the token is correct)", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
	}

	var containers []models.Container
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	for i := range containers {
		containers[i].HostID = host.ID
		containers[i].HostName = host.Name
	}

	return containers, nil
}

func (s *Scanner) startAgentContainer(ctx context.Context, host models.Host, containerID string) error {
	resp, err := s.agentRequest(ctx, host, "POST", "/api/containers/"+containerID+"/start", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent error: %s", string(body))
	}
	return nil
}

func (s *Scanner) stopAgentContainer(ctx context.Context, host models.Host, containerID string, timeout int) error {
	path := fmt.Sprintf("/api/containers/%s/stop?timeout=%d", containerID, timeout)
	resp, err := s.agentRequest(ctx, host, "POST", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent error: %s", string(body))
	}
	return nil
}

func (s *Scanner) restartAgentContainer(ctx context.Context, host models.Host, containerID string, timeout int) error {
	path := fmt.Sprintf("/api/containers/%s/restart?timeout=%d", containerID, timeout)
	resp, err := s.agentRequest(ctx, host, "POST", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent error: %s", string(body))
	}
	return nil
}

func (s *Scanner) removeAgentContainer(ctx context.Context, host models.Host, containerID string, force bool) error {
	path := fmt.Sprintf("/api/containers/%s/remove?force=%t", containerID, force)
	resp, err := s.agentRequest(ctx, host, "DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent error: %s", string(body))
	}
	return nil
}

func (s *Scanner) getAgentContainerLogs(ctx context.Context, host models.Host, containerID string, tail string) (string, error) {
	path := fmt.Sprintf("/api/containers/%s/logs?tail=%s", containerID, url.QueryEscape(tail))
	resp, err := s.agentRequest(ctx, host, "GET", path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("agent error: %s", string(body))
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result["logs"], nil
}

func (s *Scanner) listAgentImages(ctx context.Context, host models.Host) ([]imagetypes.Summary, error) {
	resp, err := s.agentRequest(ctx, host, "GET", "/api/images", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent error: %s", string(body))
	}

	var images []imagetypes.Summary
	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return images, nil
}

func (s *Scanner) removeAgentImage(ctx context.Context, host models.Host, imageID string, force bool) error {
	path := fmt.Sprintf("/api/images/%s/remove?force=%t", imageID, force)
	resp, err := s.agentRequest(ctx, host, "DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent error: %s", string(body))
	}
	return nil
}

func (s *Scanner) pruneAgentImages(ctx context.Context, host models.Host) (uint64, error) {
	resp, err := s.agentRequest(ctx, host, "POST", "/api/images/prune", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("agent error: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if space, ok := result["space_reclaimed"].(float64); ok {
		return uint64(space), nil
	}

	return 0, nil
}

func (s *Scanner) getAgentInfo(ctx context.Context, host models.Host) (*models.AgentInfo, error) {
	resp, err := s.agentRequest(ctx, host, "GET", "/info", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent error: %s", string(body))
	}

	var info models.AgentInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &info, nil
}

// Agent-specific image update operations

// imagePullTimeout is the timeout for pulling images from agents
// Large images can take several minutes to pull, so we use a generous timeout
const imagePullTimeout = 10 * time.Minute

func (s *Scanner) pullAgentImage(ctx context.Context, host models.Host, imageName string) error {
	body := map[string]string{"image": imageName}
	// Use extended timeout for image pulls - large images can take several minutes
	resp, err := s.agentRequestWithTimeout(ctx, host, "POST", "/api/images/pull", body, imagePullTimeout)
	if err != nil {
		// Provide clearer error message for timeouts
		if ctx.Err() == context.DeadlineExceeded || strings.Contains(err.Error(), "context deadline exceeded") {
			return fmt.Errorf("image pull timed out after 10 minutes - the image may be very large or the registry may be slow")
		}
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("agent does not support image pulling - please update your census-agent to the latest version")
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// pullAgentImageWithProgress pulls an image via agent with streaming progress
func (s *Scanner) pullAgentImageWithProgress(ctx context.Context, host models.Host, imageName string, onProgress PullProgressCallback) error {
	body := map[string]string{"image": imageName}

	// Request streaming response from agent
	agentURL := normalizeAgentURL(host.Address) + "/api/images/pull?stream=true"

	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", agentURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Token", host.AgentToken)
	req.Header.Set("Content-Type", "application/json")

	// Use extended timeout for streaming pull
	client := &http.Client{Timeout: imagePullTimeout}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded || strings.Contains(err.Error(), "context deadline exceeded") {
			return fmt.Errorf("image pull timed out after 10 minutes - the image may be very large or the registry may be slow")
		}
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("agent does not support image pulling - please update your census-agent to the latest version")
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode NDJSON stream from agent
	decoder := json.NewDecoder(resp.Body)
	for {
		var msg struct {
			Status         string `json:"status"`
			ID             string `json:"id"`
			ProgressDetail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}

		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode progress: %w", err)
		}

		if msg.Error != "" {
			return fmt.Errorf("pull failed: %s", msg.Error)
		}

		// Check for completion message from agent
		if msg.Status == "complete" {
			break
		}

		if onProgress != nil && msg.ID != "" {
			onProgress(msg.ID, msg.Status, msg.ProgressDetail.Current, msg.ProgressDetail.Total)
		}
	}

	return nil
}

func (s *Scanner) recreateAgentContainer(ctx context.Context, host models.Host, containerID string, dryRun bool) (*models.ContainerRecreateResult, error) {
	path := fmt.Sprintf("/api/containers/%s/recreate", containerID)
	if dryRun {
		path += "?dry_run=true"
	}

	resp, err := s.agentRequest(ctx, host, "POST", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result models.ContainerRecreateResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
