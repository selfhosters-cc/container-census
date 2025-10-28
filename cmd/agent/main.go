package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/container-census/container-census/internal/agent"
	"github.com/container-census/container-census/internal/version"
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
	if *apiToken == "" {
		*apiToken = loadOrGenerateToken(*tokenFile)
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Get version
	agentVersion := version.Get()

	// Create agent info
	agentInfo := agent.Info{
		Version:   agentVersion,
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		StartedAt: time.Now(),
	}

	log.Printf("Starting Container Census Agent v%s", agentVersion)
	log.Printf("Hostname: %s", hostname)
	log.Printf("OS: %s/%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("Docker Host: %s", *dockerHost)

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
		WriteTimeout: 30 * time.Second,
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
