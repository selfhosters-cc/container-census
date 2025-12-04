package external

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/container-census/container-census/internal/plugins/proto"
	"github.com/container-census/container-census/internal/storage"
)

// PluginStatus represents the current state of a plugin process
type PluginStatus struct {
	PluginID      string
	ProcessStatus string // "starting", "running", "stopping", "stopped", "failed"
	HealthStatus  string // "healthy", "unhealthy", "unknown"
	GRPCPort      int
	RestartCount  int
	LastRestart   time.Time
	Error         string
}

// PluginProcess represents a running plugin process
type PluginProcess struct {
	PluginID      string
	Cmd           *exec.Cmd
	GRPCClient    pb.PluginClient
	GRPCConn      *grpc.ClientConn
	GRPCPort      int
	Status        string
	HealthStatus  string
	RestartCount  int
	LastRestart   time.Time
	CancelFunc    context.CancelFunc
	StdoutLog     *CircularLog
	StderrLog     *CircularLog
}

// CircularLog stores recent log lines
type CircularLog struct {
	mu      sync.Mutex
	lines   []string
	maxSize int
	index   int
}

func NewCircularLog(maxSize int) *CircularLog {
	return &CircularLog{
		lines:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

func (cl *CircularLog) Add(line string) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if len(cl.lines) < cl.maxSize {
		cl.lines = append(cl.lines, line)
	} else {
		cl.lines[cl.index] = line
		cl.index = (cl.index + 1) % cl.maxSize
	}
}

func (cl *CircularLog) GetLines() []string {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if len(cl.lines) < cl.maxSize {
		return append([]string(nil), cl.lines...)
	}

	// Rotate to get chronological order
	result := make([]string, len(cl.lines))
	for i := 0; i < len(cl.lines); i++ {
		result[i] = cl.lines[(cl.index+i)%cl.maxSize]
	}
	return result
}

// ExternalPluginSupervisor manages external plugin processes
type ExternalPluginSupervisor struct {
	mu                sync.RWMutex
	db                *storage.DB
	processes         map[string]*PluginProcess
	censusAPIAddress  string // gRPC address for Census API
	censusAPIServer   *grpc.Server
	basePort          int // Starting port for plugin gRPC servers
	portCounter       int
	healthCheckPeriod time.Duration
	maxRestarts       int
}

// NewExternalPluginSupervisor creates a new supervisor
func NewExternalPluginSupervisor(db *storage.DB, censusAPIAddress string, basePort int) *ExternalPluginSupervisor {
	return &ExternalPluginSupervisor{
		db:                db,
		processes:         make(map[string]*PluginProcess),
		censusAPIAddress:  censusAPIAddress,
		basePort:          basePort,
		portCounter:       0,
		healthCheckPeriod: 10 * time.Second,
		maxRestarts:       3,
	}
}

// StartPlugin starts a plugin process
func (s *ExternalPluginSupervisor) StartPlugin(ctx context.Context, pluginID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already running
	if proc, exists := s.processes[pluginID]; exists {
		if proc.Status == "running" {
			return fmt.Errorf("plugin %s is already running", pluginID)
		}
		// Clean up old process
		s.stopPluginLocked(pluginID)
	}

	// Load plugin metadata from database
	plugin, err := s.db.GetExternalPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("failed to load plugin metadata: %w", err)
	}

	if plugin.BinaryPath == "" {
		return fmt.Errorf("plugin binary path not set")
	}

	// Allocate gRPC port
	grpcPort := s.basePort + s.portCounter
	s.portCounter++

	// Create process context
	procCtx, cancel := context.WithCancel(ctx)

	// Create command
	cmd := exec.CommandContext(procCtx, plugin.BinaryPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PLUGIN_ID=%s", pluginID),
		fmt.Sprintf("GRPC_PORT=%d", grpcPort),
		fmt.Sprintf("CENSUS_API=%s", s.censusAPIAddress),
	)

	// Create circular logs
	stdoutLog := NewCircularLog(100)
	stderrLog := NewCircularLog(100)

	// Capture stdout/stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start process
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start plugin process: %w", err)
	}

	// Create process record
	process := &PluginProcess{
		PluginID:     pluginID,
		Cmd:          cmd,
		GRPCPort:     grpcPort,
		Status:       "starting",
		HealthStatus: "unknown",
		RestartCount: 0,
		CancelFunc:   cancel,
		StdoutLog:    stdoutLog,
		StderrLog:    stderrLog,
	}

	s.processes[pluginID] = process

	log.Printf("[Supervisor] Started plugin %s on port %d (PID: %d)", pluginID, grpcPort, cmd.Process.Pid)

	// Start log readers
	go s.readLogLines(pluginID, stdout, stdoutLog, "stdout")
	go s.readLogLines(pluginID, stderr, stderrLog, "stderr")

	// Wait for gRPC server to be ready
	go s.waitForGRPCReady(procCtx, pluginID, grpcPort)

	// Start monitoring
	go s.monitorProcess(procCtx, pluginID)

	return nil
}

// waitForGRPCReady waits for the plugin's gRPC server to be ready
func (s *ExternalPluginSupervisor) waitForGRPCReady(ctx context.Context, pluginID string, port int) {
	timeout := 30 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Try to connect
		conn, err := grpc.NewClient(
			fmt.Sprintf("localhost:%d", port),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		client := pb.NewPluginClient(conn)

		// Try Init RPC
		initReq := &pb.InitRequest{
			PluginId:          pluginID,
			CensusApiAddress:  s.censusAPIAddress,
			CensusVersion:     "1.0.0", // TODO: Get from version package
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		resp, err := client.Init(ctx, initReq)
		cancel()

		if err == nil && resp.Success {
			s.mu.Lock()
			if proc, exists := s.processes[pluginID]; exists {
				proc.GRPCClient = client
				proc.GRPCConn = conn
				proc.Status = "running"
				proc.HealthStatus = "healthy"
			}
			s.mu.Unlock()

			log.Printf("[Supervisor] Plugin %s initialized successfully", pluginID)
			return
		}

		conn.Close()
		time.Sleep(500 * time.Millisecond)
	}

	// Timeout
	log.Printf("[Supervisor] Plugin %s failed to initialize within %v", pluginID, timeout)
	s.mu.Lock()
	if proc, exists := s.processes[pluginID]; exists {
		proc.Status = "failed"
		proc.HealthStatus = "unhealthy"
	}
	s.mu.Unlock()
}

// readLogLines reads log lines from reader
func (s *ExternalPluginSupervisor) readLogLines(pluginID string, reader interface{}, log *CircularLog, stream string) {
	// Implementation would use bufio.Scanner to read lines
	// For brevity, simplified here
}

// monitorProcess monitors plugin health and handles restarts
func (s *ExternalPluginSupervisor) monitorProcess(ctx context.Context, pluginID string) {
	ticker := time.NewTicker(s.healthCheckPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.performHealthCheck(ctx, pluginID)
		}
	}
}

// performHealthCheck checks plugin health
func (s *ExternalPluginSupervisor) performHealthCheck(ctx context.Context, pluginID string) {
	s.mu.RLock()
	proc, exists := s.processes[pluginID]
	s.mu.RUnlock()

	if !exists || proc.Status != "running" {
		return
	}

	// Check if process is still alive
	if proc.Cmd.ProcessState != nil && proc.Cmd.ProcessState.Exited() {
		log.Printf("[Supervisor] Plugin %s process exited unexpectedly", pluginID)
		s.handleProcessFailure(ctx, pluginID)
		return
	}

	// Perform gRPC healthcheck
	if proc.GRPCClient != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		resp, err := proc.GRPCClient.Healthcheck(ctx, &pb.HealthcheckRequest{
			PluginId: pluginID,
		})

		s.mu.Lock()
		if err != nil || !resp.Healthy {
			proc.HealthStatus = "unhealthy"
			log.Printf("[Supervisor] Plugin %s health check failed: %v", pluginID, err)
		} else {
			proc.HealthStatus = "healthy"
		}
		s.mu.Unlock()
	}
}

// handleProcessFailure handles plugin process failures
func (s *ExternalPluginSupervisor) handleProcessFailure(ctx context.Context, pluginID string) {
	s.mu.Lock()
	proc, exists := s.processes[pluginID]
	if !exists {
		s.mu.Unlock()
		return
	}

	proc.Status = "failed"
	proc.RestartCount++
	proc.LastRestart = time.Now()

	shouldRestart := proc.RestartCount <= s.maxRestarts
	s.mu.Unlock()

	if shouldRestart {
		log.Printf("[Supervisor] Attempting to restart plugin %s (attempt %d/%d)",
			pluginID, proc.RestartCount, s.maxRestarts)

		// Wait before restart
		time.Sleep(2 * time.Second)

		// Restart
		if err := s.StartPlugin(ctx, pluginID); err != nil {
			log.Printf("[Supervisor] Failed to restart plugin %s: %v", pluginID, err)
		}
	} else {
		log.Printf("[Supervisor] Plugin %s exceeded max restart attempts (%d), giving up",
			pluginID, s.maxRestarts)

		// Disable plugin in database
		if err := s.db.SetPluginEnabled(pluginID, false); err != nil {
			log.Printf("[Supervisor] Failed to disable plugin %s: %v", pluginID, err)
		}
	}
}

// StopPlugin gracefully stops a plugin
func (s *ExternalPluginSupervisor) StopPlugin(pluginID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopPluginLocked(pluginID)
}

func (s *ExternalPluginSupervisor) stopPluginLocked(pluginID string) error {
	proc, exists := s.processes[pluginID]
	if !exists {
		return fmt.Errorf("plugin %s is not running", pluginID)
	}

	log.Printf("[Supervisor] Stopping plugin %s", pluginID)
	proc.Status = "stopping"

	// Call Stop RPC if available
	if proc.GRPCClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		proc.GRPCClient.Stop(ctx, &pb.StopRequest{PluginId: pluginID})
	}

	// Close gRPC connection
	if proc.GRPCConn != nil {
		proc.GRPCConn.Close()
	}

	// Cancel context to stop monitors
	if proc.CancelFunc != nil {
		proc.CancelFunc()
	}

	// Kill process if still running
	if proc.Cmd.Process != nil && proc.Cmd.ProcessState == nil {
		proc.Cmd.Process.Kill()
	}

	proc.Status = "stopped"
	delete(s.processes, pluginID)

	log.Printf("[Supervisor] Plugin %s stopped", pluginID)
	return nil
}

// RestartPlugin restarts a plugin
func (s *ExternalPluginSupervisor) RestartPlugin(ctx context.Context, pluginID string) error {
	if err := s.StopPlugin(pluginID); err != nil {
		log.Printf("[Supervisor] Error stopping plugin %s for restart: %v", pluginID, err)
	}

	time.Sleep(1 * time.Second)
	return s.StartPlugin(ctx, pluginID)
}

// GetPluginStatus returns the status of a plugin
func (s *ExternalPluginSupervisor) GetPluginStatus(pluginID string) (PluginStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	proc, exists := s.processes[pluginID]
	if !exists {
		return PluginStatus{
			PluginID:      pluginID,
			ProcessStatus: "stopped",
			HealthStatus:  "unknown",
		}, nil
	}

	return PluginStatus{
		PluginID:      proc.PluginID,
		ProcessStatus: proc.Status,
		HealthStatus:  proc.HealthStatus,
		GRPCPort:      proc.GRPCPort,
		RestartCount:  proc.RestartCount,
		LastRestart:   proc.LastRestart,
	}, nil
}

// GetPluginLogs returns recent log lines
func (s *ExternalPluginSupervisor) GetPluginLogs(pluginID string) (stdout, stderr []string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	proc, exists := s.processes[pluginID]
	if !exists {
		return nil, nil, fmt.Errorf("plugin %s is not running", pluginID)
	}

	return proc.StdoutLog.GetLines(), proc.StderrLog.GetLines(), nil
}

// GetGRPCClient returns the gRPC client for a plugin
func (s *ExternalPluginSupervisor) GetGRPCClient(pluginID string) (pb.PluginClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	proc, exists := s.processes[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin %s is not running", pluginID)
	}

	if proc.GRPCClient == nil {
		return nil, fmt.Errorf("plugin %s gRPC client not ready", pluginID)
	}

	return proc.GRPCClient, nil
}

// StopAll stops all running plugins
func (s *ExternalPluginSupervisor) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("[Supervisor] Stopping all plugins...")

	for pluginID := range s.processes {
		s.stopPluginLocked(pluginID)
	}
}
