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

	"github.com/container-census/container-census/internal/models"
	"github.com/docker/docker/api/types"
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

	client := &http.Client{Timeout: s.timeout}
	return client.Do(req)
}

func (s *Scanner) scanAgentHost(ctx context.Context, host models.Host) ([]models.Container, error) {
	resp, err := s.agentRequest(ctx, host, "GET", "/api/containers", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
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

func (s *Scanner) listAgentImages(ctx context.Context, host models.Host) ([]types.ImageSummary, error) {
	resp, err := s.agentRequest(ctx, host, "GET", "/api/images", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent error: %s", string(body))
	}

	var images []types.ImageSummary
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
