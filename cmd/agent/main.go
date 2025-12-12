package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/selfhosters-cc/container-census/internal/agent"
	"github.com/selfhosters-cc/container-census/internal/version"
)

func main() {
	// Command line flags
	port := flag.Int("port", 9876, "Port to listen on")
	apiToken := flag.String("token", "", "API token for authentication")
	serverURL := flag.String("server", "", "Optional: URL of the central server to register with")
	dockerHost := flag.String("docker-host", "unix:///var/run/docker.sock", "Docker daemon host")
	tokenFile := flag.String("token-file", "/app/data/agent-token", "Path to token file for persistence")

	flag.Parse()

	// Load or generate token
	// Priority: 1. Command-line flag, 2. Environment variable, 3. File, 4. Generate new
	if *apiToken == "" {
		// Check environment variable first
		if envToken := os.Getenv("API_TOKEN"); envToken != "" {
			*apiToken = envToken
			log.Printf("Using API token from API_TOKEN environment variable")
		} else {
			*apiToken = loadOrGenerateToken(*tokenFile)
		}
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Get version and build time
	agentVersion := version.Get()
	buildTime := version.GetBuildTime()

	// Check Trivy availability
	hasTrivyBool, trivyVer := checkTrivyAvailability()

	// Get Trivy DB metadata if Trivy is available
	var trivyDBUpdatedAt string
	if hasTrivyBool {
		trivyDBUpdatedAt = agent.GetTrivyDBMetadata("/app/data/.trivy")
	}

	// Create agent info
	agentInfo := agent.Info{
		Version:         agentVersion,
		BuildTime:       buildTime,
		Hostname:        hostname,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		StartedAt:       time.Now(),
		HasTrivy:        hasTrivyBool,
		TrivyVersion:    trivyVer,
		TrivyDBUpdatedAt: trivyDBUpdatedAt,
	}

	log.Printf("Starting Container Census Agent v%s", agentVersion)
	if buildTime != "" && buildTime != "unknown" {
		log.Printf("Build Time: %s", buildTime)
	}
	log.Printf("Hostname: %s", hostname)
	log.Printf("OS: %s/%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("Docker Host: %s", *dockerHost)
	if hasTrivyBool {
		log.Printf("Trivy: v%s (vulnerability scanning enabled)", trivyVer)
	} else {
		log.Printf("Trivy: not available (vulnerability scanning disabled)")
	}

	// Create agent server
	agentServer, err := agent.New(*dockerHost, *apiToken, agentInfo)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Register with central server if URL provided
	if *serverURL != "" {
		go func() {
			if err := agentServer.RegisterWithServer(*serverURL); err != nil {
				log.Printf("Failed to register with server: %v", err)
			} else {
				log.Printf("Successfully registered with server: %s", *serverURL)
			}
		}()
	}

	// HTTP server
	addr := fmt.Sprintf(":%d", *port)
	server := &http.Server{
		Addr:         addr,
		Handler:      agentServer.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 2 * time.Minute, // Increased for stats collection on hosts with many containers
		IdleTimeout:  60 * time.Second,
	}

	// Check for updates on startup
	go checkForUpdates()

	// Create context for background tasks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start daily version check
	go runDailyVersionCheck(ctx)

	// Start server
	go func() {
		log.Printf("Agent listening on http://0.0.0.0%s", addr)
		log.Printf("Health check: http://localhost%s/health", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancel() // Cancel background tasks

	log.Println("Shutting down agent...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Agent forced to shutdown: %v", err)
	}

	log.Println("Agent stopped")
}

// checkForUpdates checks for new versions and logs a warning if an update is available
func checkForUpdates() {
	info := version.CheckLatestVersion()

	if info.Error != nil {
		// Silently ignore errors during version check
		log.Printf("Version check: %v", info.Error)
		return
	}

	if info.UpdateAvailable {
		log.Printf("⚠️  UPDATE AVAILABLE: Container Census Agent %s → %s", info.CurrentVersion, info.LatestVersion)
		log.Printf("   Download: %s", info.ReleaseURL)
	}
}

// runDailyVersionCheck performs version checks once per day
func runDailyVersionCheck(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkForUpdates()
		}
	}
}

// loadOrGenerateToken loads a token from file or generates a new one if it doesn't exist
func loadOrGenerateToken(tokenFile string) string {
	// Try to read existing token
	if data, err := os.ReadFile(tokenFile); err == nil {
		token := string(data)
		if len(token) > 0 {
			log.Printf("Using existing API token from %s", tokenFile)
			log.Printf("API Token: %s", token)
			return token
		}
	}

	// Generate new token
	token := agent.GenerateToken()
	log.Printf("Generated new API token: %s", token)
	log.Println("IMPORTANT: Save this token - you'll need it to add this agent to the server")

	// Try to persist token for future restarts
	if err := os.MkdirAll(filepath.Dir(tokenFile), 0755); err != nil {
		log.Printf("Warning: Could not create token directory: %v", err)
		log.Println("Token will not persist across restarts. Mount a volume at /app/data to enable persistence.")
		return token
	}

	if err := os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		log.Printf("Warning: Could not save token to file: %v", err)
		log.Println("Token will not persist across restarts. Mount a volume at /app/data to enable persistence.")
	} else {
		log.Printf("Token saved to %s (will persist across restarts)", tokenFile)
	}

	return token
}

// checkTrivyAvailability checks if Trivy is installed and returns version
func checkTrivyAvailability() (bool, string) {
	cmd := exec.Command("trivy", "--version")
	output, err := cmd.Output()
	if err != nil {
		return false, ""
	}

	// Parse version from output
	// Expected format: "Version: 0.58.1"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			return true, version
		}
	}
	return true, "unknown"
}
