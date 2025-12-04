package external

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/container-census/container-census/internal/storage"
	"gopkg.in/yaml.v3"
)

// PluginManifest represents the plugin.yaml structure
type PluginManifest struct {
	ID              string            `yaml:"id"`
	Name            string            `yaml:"name"`
	Version         string            `yaml:"version"`
	Description     string            `yaml:"description"`
	Author          string            `yaml:"author"`
	Repository      string            `yaml:"repository"`
	MinCensusVersion string           `yaml:"min_census_version"`
	Permissions     []string          `yaml:"permissions"`
	Frontend        FrontendConfig    `yaml:"frontend"`
	Tab             TabConfig         `yaml:"tab"`
}

type FrontendConfig struct {
	BundleURL string `yaml:"bundle_url"`
	CSSURL    string `yaml:"css_url"`
	InitFunc  string `yaml:"init_function"`
}

type TabConfig struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label"`
	Icon  string `yaml:"icon"`
	Order int    `yaml:"order"`
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// PluginInstaller handles plugin installation from GitHub
type PluginInstaller struct {
	db          *storage.DB
	pluginsDir  string
	httpClient  *http.Client
}

// NewPluginInstaller creates a new plugin installer
func NewPluginInstaller(db *storage.DB, pluginsDir string) *PluginInstaller {
	return &PluginInstaller{
		db:         db,
		pluginsDir: pluginsDir,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Install installs a plugin from a GitHub repository URL
func (i *PluginInstaller) Install(ctx context.Context, repoURL, version string) error {
	log.Printf("[Installer] Installing plugin from %s (version: %s)", repoURL, version)

	// Parse repository owner and name from URL
	owner, repo, err := parseGitHubURL(repoURL)
	if err != nil {
		return fmt.Errorf("invalid GitHub URL: %w", err)
	}

	// Fetch plugin manifest
	manifest, err := i.fetchPluginManifest(owner, repo)
	if err != nil {
		return fmt.Errorf("failed to fetch plugin manifest: %w", err)
	}

	log.Printf("[Installer] Found plugin: %s v%s", manifest.Name, manifest.Version)

	// Check if plugin is already installed
	existing, err := i.db.GetExternalPlugin(manifest.ID)
	if err != nil {
		return fmt.Errorf("failed to check existing plugin: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("plugin %s is already installed (version %s)", manifest.ID, existing.Version)
	}

	// Determine which version to install
	targetVersion := version
	if targetVersion == "" || targetVersion == "latest" {
		release, err := i.getLatestRelease(owner, repo)
		if err != nil {
			return fmt.Errorf("failed to get latest release: %w", err)
		}
		targetVersion = release.TagName
	}

	// Create plugin directory
	pluginDir := filepath.Join(i.pluginsDir, manifest.ID)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download and extract plugin binary
	binaryPath, err := i.downloadBinary(owner, repo, targetVersion, pluginDir)
	if err != nil {
		os.RemoveAll(pluginDir) // Clean up on error
		return fmt.Errorf("failed to download plugin binary: %w", err)
	}

	// Download frontend assets if specified
	frontendDir := filepath.Join(pluginDir, "frontend")
	if manifest.Frontend.BundleURL != "" {
		if err := os.MkdirAll(frontendDir, 0755); err != nil {
			os.RemoveAll(pluginDir)
			return fmt.Errorf("failed to create frontend directory: %w", err)
		}

		if err := i.downloadFrontendAssets(owner, repo, targetVersion, frontendDir); err != nil {
			os.RemoveAll(pluginDir)
			return fmt.Errorf("failed to download frontend assets: %w", err)
		}
	}

	// Save plugin record to database
	// Marshal tab config to JSON
	tabConfigJSON := ""
	if manifest.Tab.ID != "" {
		tabBytes, err := json.Marshal(manifest.Tab)
		if err != nil {
			log.Printf("[Installer] Warning: failed to marshal tab config: %v", err)
		} else {
			tabConfigJSON = string(tabBytes)
		}
	}

	record := &storage.ExternalPluginRecord{
		PluginRecord: storage.PluginRecord{
			ID:          manifest.ID,
			Name:        manifest.Name,
			Version:     targetVersion,
			SourceType:  "github",
			SourceURL:   repoURL,
			Enabled:     true,
			InstalledAt: time.Now(),
			UpdatedAt:   time.Now(),
			TabConfig:   tabConfigJSON,
		},
		BinaryPath:     binaryPath,
		Permissions:    manifest.Permissions,
		FrontendBundle: filepath.Join(frontendDir, "bundle.js"),
		FrontendCSS:    filepath.Join(frontendDir, "bundle.css"),
		ProcessStatus:  "stopped",
	}

	if err := i.db.SaveExternalPlugin(record); err != nil {
		os.RemoveAll(pluginDir)
		return fmt.Errorf("failed to save plugin record: %w", err)
	}

	log.Printf("[Installer] Successfully installed plugin %s v%s", manifest.ID, targetVersion)
	return nil
}

// Update updates a plugin to the latest version
func (i *PluginInstaller) Update(ctx context.Context, pluginID string) error {
	log.Printf("[Installer] Updating plugin %s", pluginID)

	// Get existing plugin
	existing, err := i.db.GetExternalPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("failed to get plugin: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	// Uninstall old version
	if err := i.Uninstall(pluginID); err != nil {
		return fmt.Errorf("failed to uninstall old version: %w", err)
	}

	// Reinstall latest version
	return i.Install(ctx, existing.SourceURL, "latest")
}

// Uninstall removes a plugin
func (i *PluginInstaller) Uninstall(pluginID string) error {
	log.Printf("[Installer] Uninstalling plugin %s", pluginID)

	// Get plugin record
	plugin, err := i.db.GetExternalPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("failed to get plugin: %w", err)
	}
	if plugin == nil {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	// Remove plugin directory
	pluginDir := filepath.Join(i.pluginsDir, pluginID)
	if err := os.RemoveAll(pluginDir); err != nil {
		log.Printf("[Installer] Warning: failed to remove plugin directory: %v", err)
	}

	// Delete from database
	if err := i.db.DeletePlugin(pluginID); err != nil {
		return fmt.Errorf("failed to delete plugin from database: %w", err)
	}

	log.Printf("[Installer] Successfully uninstalled plugin %s", pluginID)
	return nil
}

// fetchPluginManifest fetches the plugin.yaml from GitHub
func (i *PluginInstaller) fetchPluginManifest(owner, repo string) (*PluginManifest, error) {
	// Fetch raw plugin.yaml from main branch
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/plugin.yaml", owner, repo)

	resp, err := i.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch manifest: HTTP %d", resp.StatusCode)
	}

	var manifest PluginManifest
	if err := yaml.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// getLatestRelease gets the latest GitHub release
func (i *PluginInstaller) getLatestRelease(owner, repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	resp, err := i.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch releases: HTTP %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// downloadBinary downloads the plugin binary for the current platform
func (i *PluginInstaller) downloadBinary(owner, repo, version, destDir string) (string, error) {
	// Determine platform-specific binary name (use hyphen to match common naming convention)
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)

	// Get release
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, version)
	resp, err := i.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release: %w", err)
	}

	// Find matching asset
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, platform) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return "", fmt.Errorf("no binary found for platform %s", platform)
	}

	// Download binary
	log.Printf("[Installer] Downloading binary from %s", downloadURL)
	resp, err = i.httpClient.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	// Save to file
	binaryPath := filepath.Join(destDir, "plugin")
	file, err := os.OpenFile(binaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create binary file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write binary: %w", err)
	}

	log.Printf("[Installer] Binary saved to %s", binaryPath)
	return binaryPath, nil
}

// downloadFrontendAssets downloads frontend bundle and CSS
func (i *PluginInstaller) downloadFrontendAssets(owner, repo, version, destDir string) error {
	// Get release to find frontend assets
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, version)
	resp, err := i.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release: %w", err)
	}

	// Download bundle.js if present
	for _, asset := range release.Assets {
		if asset.Name == "bundle.js" {
			log.Printf("[Installer] Downloading frontend bundle from %s", asset.BrowserDownloadURL)
			if err := i.downloadFile(asset.BrowserDownloadURL, filepath.Join(destDir, "bundle.js")); err != nil {
				return fmt.Errorf("failed to download JS bundle: %w", err)
			}
		}
		// Download bundle.css if present
		if asset.Name == "bundle.css" {
			log.Printf("[Installer] Downloading frontend CSS from %s", asset.BrowserDownloadURL)
			if err := i.downloadFile(asset.BrowserDownloadURL, filepath.Join(destDir, "bundle.css")); err != nil {
				return fmt.Errorf("failed to download CSS: %w", err)
			}
		}
	}

	return nil
}

// downloadFile downloads a file from a URL
func (i *PluginInstaller) downloadFile(url, destPath string) error {
	log.Printf("[Installer] Downloading %s to %s", url, destPath)

	resp, err := i.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}

	return nil
}

// extractTarGz extracts a .tar.gz archive
func (i *PluginInstaller) extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			file, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}
	}

	return nil
}

// parseGitHubURL extracts owner and repo from a GitHub URL
func parseGitHubURL(url string) (owner, repo string, err error) {
	// Handle various GitHub URL formats:
	// https://github.com/owner/repo
	// https://github.com/owner/repo.git
	// github.com/owner/repo

	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "www.")
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")

	parts := strings.Split(url, "/")
	if len(parts) < 3 || parts[0] != "github.com" {
		return "", "", fmt.Errorf("invalid GitHub URL format")
	}

	return parts[1], parts[2], nil
}
