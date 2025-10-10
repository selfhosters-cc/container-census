package version

import (
	"os"
	"strings"
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
