package api

import (
	"encoding/json"
	"net/http"

	"github.com/container-census/container-census/internal/auth"
)

// LoginRequest represents the login request payload
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin validates credentials and creates a session
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate credentials against environment variables
	if req.Username != s.authConfig.Username || req.Password != s.authConfig.Password {
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Create session cookie
	if err := auth.CreateSession(w, r); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleLogout destroys the current session
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := auth.DestroySession(w, r); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to destroy session")
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"success": true})
}
