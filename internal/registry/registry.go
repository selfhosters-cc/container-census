package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ImageUpdateInfo contains information about an image update check
type ImageUpdateInfo struct {
	Available     bool      `json:"available"`
	LocalDigest   string    `json:"local_digest"`
	RemoteDigest  string    `json:"remote_digest"`
	RemoteCreated time.Time `json:"remote_created,omitempty"`
	ImageName     string    `json:"image_name"`
	Tag           string    `json:"tag"`
}

// ManifestResponse represents a Docker registry manifest response
type ManifestResponse struct {
	SchemaVersion int                    `json:"schemaVersion"`
	MediaType     string                 `json:"mediaType"`
	Config        ConfigDescriptor       `json:"config"`
	Layers        []LayerDescriptor      `json:"layers"`
	Manifests     []ManifestDescriptor   `json:"manifests,omitempty"` // For manifest lists
}

// ConfigDescriptor represents the config section of a manifest
type ConfigDescriptor struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

// LayerDescriptor represents a layer in the manifest
type LayerDescriptor struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

// ManifestDescriptor represents a platform-specific manifest in a manifest list
type ManifestDescriptor struct {
	MediaType string   `json:"mediaType"`
	Size      int64    `json:"size"`
	Digest    string   `json:"digest"`
	Platform  Platform `json:"platform"`
}

// Platform represents the platform of a manifest
type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

// TokenResponse represents a Docker registry token response
type TokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// Client is a Docker registry client
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new registry client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CheckImageUpdate checks if a newer version of an image is available in the registry
func (c *Client) CheckImageUpdate(ctx context.Context, imageName string, localDigest string) (*ImageUpdateInfo, error) {
	// Parse the image name
	registry, repository, tag, err := parseImageName(imageName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image name: %w", err)
	}

	// Only check images with :latest tag
	if tag != "latest" {
		return &ImageUpdateInfo{
			Available:   false,
			LocalDigest: localDigest,
			ImageName:   imageName,
			Tag:         tag,
		}, nil
	}

	// Get the remote digest
	remoteDigest, err := c.getImageDigest(ctx, registry, repository, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote digest: %w", err)
	}

	// Normalize digests for comparison (remove sha256: prefix if present)
	normalizedLocal := normalizeDigest(localDigest)
	normalizedRemote := normalizeDigest(remoteDigest)

	// Compare digests
	available := normalizedLocal != normalizedRemote

	return &ImageUpdateInfo{
		Available:    available,
		LocalDigest:  normalizedLocal,
		RemoteDigest: normalizedRemote,
		ImageName:    imageName,
		Tag:          tag,
	}, nil
}

// getImageDigest retrieves the digest of an image from the registry
func (c *Client) getImageDigest(ctx context.Context, registry, repository, tag string) (string, error) {
	// Get auth token if needed
	token, err := c.getAuthToken(ctx, registry, repository)
	if err != nil {
		return "", fmt.Errorf("failed to get auth token: %w", err)
	}

	// Construct manifest URL
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repository, tag)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", manifestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	// Get digest from Docker-Content-Digest header
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest != "" {
		return digest, nil
	}

	// If no header, parse the manifest body to get config digest
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest body: %w", err)
	}

	var manifest ManifestResponse
	if err := json.Unmarshal(body, &manifest); err != nil {
		return "", fmt.Errorf("failed to parse manifest: %w", err)
	}

	// If it's a manifest list, we can't easily get a single digest
	// Return the digest of the first manifest or the list digest
	if len(manifest.Manifests) > 0 {
		// For manifest lists, use the digest from the response header or compute it
		return digest, nil
	}

	// For regular manifests, return the config digest
	if manifest.Config.Digest != "" {
		return manifest.Config.Digest, nil
	}

	return "", fmt.Errorf("no digest found in manifest response")
}

// getAuthToken retrieves an authentication token for the registry
func (c *Client) getAuthToken(ctx context.Context, registry, repository string) (string, error) {
	// Only Docker Hub requires token auth in most cases
	// For other registries, we'll try without a token first

	if !strings.Contains(registry, "docker.io") && registry != "registry-1.docker.io" {
		// For non-Docker Hub registries, try without auth first
		return "", nil
	}

	// For Docker Hub, get an anonymous token
	authURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repository)

	req, err := http.NewRequestWithContext(ctx, "GET", authURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Token auth failed, but we can try without it
		return "", nil
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.Token != "" {
		return tokenResp.Token, nil
	}

	return tokenResp.AccessToken, nil
}

// parseImageName parses a Docker image name into its components
// Returns: registry, repository, tag, error
func parseImageName(imageName string) (string, string, string, error) {
	// Default values
	registry := "registry-1.docker.io"
	tag := "latest"

	// Remove any leading/trailing whitespace
	imageName = strings.TrimSpace(imageName)

	// Split by tag separator
	parts := strings.Split(imageName, ":")
	nameWithoutTag := parts[0]
	if len(parts) > 1 {
		tag = parts[len(parts)-1]
	}

	// Check if there's a registry specified (contains a dot or port)
	nameParts := strings.Split(nameWithoutTag, "/")

	var repository string

	if len(nameParts) >= 2 && (strings.Contains(nameParts[0], ".") || strings.Contains(nameParts[0], ":")) {
		// Custom registry specified
		registry = nameParts[0]
		repository = strings.Join(nameParts[1:], "/")
	} else {
		// Docker Hub - check if it's an official image
		if len(nameParts) == 1 {
			// Official image (e.g., "nginx")
			repository = "library/" + nameParts[0]
		} else {
			// User image (e.g., "user/image")
			repository = nameWithoutTag
		}
	}

	return registry, repository, tag, nil
}

// normalizeDigest removes the "sha256:" prefix if present
func normalizeDigest(digest string) string {
	digest = strings.TrimSpace(digest)

	// Remove sha256: prefix
	if strings.HasPrefix(digest, "sha256:") {
		digest = strings.TrimPrefix(digest, "sha256:")
	}

	// If the digest is very long (full docker image ID), take only the first 12 chars
	// to match Docker's truncated format
	if len(digest) > 64 {
		digest = digest[:12]
	}

	return digest
}
