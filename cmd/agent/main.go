package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	flag.Parse()

	// Generate random token if not provided
	if *apiToken == "" {
		*apiToken = agent.GenerateToken()
		log.Printf("Generated API token: %s", *apiToken)
		log.Println("IMPORTANT: Save this token - you'll need it to add this agent to the server")
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

	// Start server
	go func() {
		log.Printf("Agent listening on http://0.0.0.0%s", addr)
		log.Printf("Health check: http://localhost%s/health", addr)
		log.Printf("API Token: %s", *apiToken)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down agent...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Agent forced to shutdown: %v", err)
	}

	log.Println("Agent stopped")
}
