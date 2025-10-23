package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// Version is read from .version file or defaults to "dev"
	Version string
)

func init() {
	Version = loadVersion()
}

// loadVersion reads the version from .version file
func loadVersion() string {
	// Try multiple possible locations for .version file
	locations := []string{
		".version",
		"../.version",
		"../../.version",
		"../../../.version",
	}

	for _, loc := range locations {
		if data, err := os.ReadFile(loc); err == nil {
			v := strings.TrimSpace(string(data))
			if v != "" {
				return v
			}
		}
	}

	// Fallback to environment variable
	if v := os.Getenv("VERSION"); v != "" {
		return v
	}

	// Default to "dev" if no version found
	return "dev"
}

// Get returns the current version
func Get() string {
	return Version
}

// GitHubRelease represents the GitHub API response for a release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Name    string `json:"name"`
}

// UpdateInfo contains information about available updates
type UpdateInfo struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	ReleaseURL      string
	CheckedAt       time.Time
	Error           error
}

var (
	// Cache for version check results
	cachedUpdateInfo *UpdateInfo
	cacheMutex       sync.RWMutex
	cacheExpiration  = 24 * time.Hour
)

// CheckLatestVersion checks GitHub Releases API for the latest version
// Results are cached for 24 hours to respect rate limits
func CheckLatestVersion() *UpdateInfo {
	// Check cache first
	cacheMutex.RLock()
	if cachedUpdateInfo != nil && time.Since(cachedUpdateInfo.CheckedAt) < cacheExpiration {
		cacheMutex.RUnlock()
		return cachedUpdateInfo
	}
	cacheMutex.RUnlock()

	// Fetch latest version from GitHub
	info := &UpdateInfo{
		CurrentVersion: Version,
		CheckedAt:      time.Now(),
	}

	// Make HTTP request to GitHub API
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/selfhosters-cc/container-census/releases/latest")
	if err != nil {
		info.Error = fmt.Errorf("failed to fetch latest version: %w", err)
		cacheResult(info)
		return info
	}
	defer resp.Body.Close()

	// Handle 404 (no releases yet)
	if resp.StatusCode == 404 {
		info.Error = fmt.Errorf("no releases found")
		cacheResult(info)
		return info
	}

	if resp.StatusCode != 200 {
		info.Error = fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
		cacheResult(info)
		return info
	}

	// Parse response
	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		info.Error = fmt.Errorf("failed to parse GitHub API response: %w", err)
		cacheResult(info)
		return info
	}

	// Remove 'v' prefix from tag if present
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	info.LatestVersion = latestVersion
	info.ReleaseURL = release.HTMLURL

	// Compare versions
	info.UpdateAvailable = isNewerVersion(latestVersion, Version)

	// Cache the result
	cacheResult(info)

	return info
}

// cacheResult stores the update info in cache
func cacheResult(info *UpdateInfo) {
	cacheMutex.Lock()
	cachedUpdateInfo = info
	cacheMutex.Unlock()
}

// isNewerVersion compares two semantic versions (X.Y.Z format)
// Returns true if newVer is newer than currentVer
func isNewerVersion(newVer, currentVer string) bool {
	// Handle "dev" version
	if currentVer == "dev" {
		return false // Don't show updates for dev builds
	}

	// Parse versions
	newParts := parseVersion(newVer)
	currentParts := parseVersion(currentVer)

	// Compare major.minor.patch
	for i := 0; i < 3; i++ {
		if newParts[i] > currentParts[i] {
			return true
		}
		if newParts[i] < currentParts[i] {
			return false
		}
	}

	return false // Versions are equal
}

// parseVersion splits a version string into [major, minor, patch]
func parseVersion(ver string) [3]int {
	parts := strings.Split(ver, ".")
	result := [3]int{0, 0, 0}

	for i := 0; i < len(parts) && i < 3; i++ {
		if num, err := strconv.Atoi(parts[i]); err == nil {
			result[i] = num
		}
	}

	return result
}

// GetUpdateInfo returns cached update information or checks if cache is stale
func GetUpdateInfo() *UpdateInfo {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	if cachedUpdateInfo == nil {
		return nil
	}

	return cachedUpdateInfo
}

// InvalidateCache forces a fresh version check on the next call to CheckLatestVersion
func InvalidateCache() {
	cacheMutex.Lock()
	cachedUpdateInfo = nil
	cacheMutex.Unlock()
}
