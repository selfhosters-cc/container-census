package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/gorilla/mux"
)

// detectHostType determines host type from address
func detectHostType(address string) string {
	switch {
	case strings.HasPrefix(address, "agent://"), strings.HasPrefix(address, "http://"), strings.HasPrefix(address, "https://"):
		return "agent"
	case strings.HasPrefix(address, "unix://"):
		return "unix"
	case strings.HasPrefix(address, "tcp://"):
		return "tcp"
	case strings.HasPrefix(address, "ssh://"):
		return "ssh"
	case address == "" || address == "local":
		return "unix"
	default:
		return "unknown"
	}
}

// handleAddAgentHost adds a new agent-based host
func (s *Server) handleAddAgentHost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Address     string `json:"address"`
		Description string `json:"description"`
		AgentToken  string `json:"agent_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate required fields
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.Address == "" {
		respondError(w, http.StatusBadRequest, "Address is required")
		return
	}
	if req.AgentToken == "" {
		respondError(w, http.StatusBadRequest, "Agent token is required")
		return
	}

	// Detect host type
	hostType := detectHostType(req.Address)

	// Verify agent connectivity
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	host := models.Host{
		Name:        req.Name,
		Address:     req.Address,
		Description: req.Description,
		HostType:    hostType,
		AgentToken:  req.AgentToken,
		AgentStatus: "unknown",
		Enabled:     true,
	}

	// Try to ping the agent
	if hostType == "agent" {
		if err := s.verifyAgentConnection(ctx, host); err != nil {
			respondError(w, http.StatusBadGateway, "Failed to connect to agent: "+err.Error())
			return
		}
		host.AgentStatus = "online"
		host.LastSeen = time.Now()
	}

	// Add to database
	id, err := s.db.AddHost(host)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to add host: "+err.Error())
		return
	}

	host.ID = id
	respondJSON(w, http.StatusCreated, host)
}

// verifyAgentConnection checks if an agent is reachable
func (s *Server) verifyAgentConnection(ctx context.Context, host models.Host) error {
	// Use the scanner's verification (we'll need to expose this)
	return s.scanner.VerifyConnection(ctx, host.Address)
}

// handleTestAgentConnection tests connectivity to an agent without adding it
func (s *Server) handleTestAgentConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address    string `json:"address"`
		AgentToken string `json:"agent_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	host := models.Host{
		Address:    req.Address,
		AgentToken: req.AgentToken,
	}

	if err := s.verifyAgentConnection(ctx, host); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Agent is reachable",
	})
}

// handleGetAgentInfo retrieves agent information
func (s *Server) handleGetAgentInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	host, err := s.db.GetHost(hostID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	if host.HostType != "agent" {
		respondError(w, http.StatusBadRequest, "Host is not an agent")
		return
	}

	// Get agent info from the /info endpoint
	ctx := r.Context()
	info, err := s.getAgentInfo(ctx, *host)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get agent info: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, info)
}

// getAgentInfo fetches agent information
func (s *Server) getAgentInfo(ctx context.Context, host models.Host) (*models.AgentInfo, error) {
	// We would need to add this method to the scanner or create an HTTP client here
	// For now, return a placeholder
	return &models.AgentInfo{
		Hostname: host.Name,
	}, nil
}
