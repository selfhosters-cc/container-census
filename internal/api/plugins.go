package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/plugins"
	"github.com/gorilla/mux"
)

// SetPluginManager sets the plugin manager
func (s *Server) SetPluginManager(pm *plugins.Manager) {
	s.pluginManager = pm
}

// GetRouter returns the API subrouter for plugin route mounting (with auth middleware)
func (s *Server) GetRouter() *mux.Router {
	return s.apiRouter
}

// setupPluginRoutes sets up routes for plugin management
func (s *Server) setupPluginRoutes(api *mux.Router) {
	// Plugin management endpoints
	api.HandleFunc("/plugins", s.handleGetPlugins).Methods("GET")
	api.HandleFunc("/plugins/tabs", s.handleGetPluginTabs).Methods("GET")
	api.HandleFunc("/plugins/badges", s.handleGetPluginBadges).Methods("GET")
	api.HandleFunc("/plugins/{id}", s.handleGetPlugin).Methods("GET")
	api.HandleFunc("/plugins/{id}/enable", s.handleEnablePlugin).Methods("PUT")
	api.HandleFunc("/plugins/{id}/disable", s.handleDisablePlugin).Methods("PUT")
	api.HandleFunc("/plugins/{id}/settings", s.handleGetPluginSettings).Methods("GET")
	api.HandleFunc("/plugins/{id}/settings", s.handleUpdatePluginSettings).Methods("PUT")
}

// handleGetPlugins returns all registered plugins
func (s *Server) handleGetPlugins(w http.ResponseWriter, r *http.Request) {
	if s.pluginManager == nil {
		respondJSON(w, http.StatusOK, []interface{}{})
		return
	}

	infos, err := s.pluginManager.GetAllPluginInfo()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Add enabled status from database
	type PluginWithStatus struct {
		plugins.PluginInfo
		Enabled bool `json:"enabled"`
	}

	result := make([]PluginWithStatus, len(infos))
	for i, info := range infos {
		// For built-in plugins, check if loaded
		// For external plugins, check database
		enabled := false
		if info.BuiltIn {
			_, loaded := s.pluginManager.GetPlugin(info.ID)
			enabled = loaded
		} else {
			// External plugin - get enabled status from database
			record, err := s.db.GetPlugin(info.ID)
			if err == nil && record != nil {
				enabled = record.Enabled
			}
		}

		result[i] = PluginWithStatus{
			PluginInfo: info,
			Enabled:    enabled,
		}
	}

	respondJSON(w, http.StatusOK, result)
}

// handleGetPlugin returns a specific plugin
func (s *Server) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	if s.pluginManager == nil {
		respondError(w, http.StatusServiceUnavailable, "Plugin system not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	plugin, ok := s.pluginManager.GetPlugin(id)
	if !ok {
		respondError(w, http.StatusNotFound, "Plugin not found")
		return
	}

	info := plugin.Info()

	// Get settings definition if available
	type PluginDetails struct {
		plugins.PluginInfo
		Enabled  bool                        `json:"enabled"`
		Settings *plugins.SettingsDefinition `json:"settings,omitempty"`
		Tab      *plugins.TabDefinition      `json:"tab,omitempty"`
	}

	details := PluginDetails{
		PluginInfo: info,
		Enabled:    true, // If we got it, it's enabled
		Settings:   plugin.Settings(),
		Tab:        plugin.Tab(),
	}

	respondJSON(w, http.StatusOK, details)
}

// handleGetPluginTabs returns all plugin tabs for navigation
func (s *Server) handleGetPluginTabs(w http.ResponseWriter, r *http.Request) {
	if s.pluginManager == nil {
		respondJSON(w, http.StatusOK, []interface{}{})
		return
	}

	tabs := s.pluginManager.GetAllTabs()
	respondJSON(w, http.StatusOK, tabs)
}

// handleGetPluginBadges returns badges for a container from all plugins
func (s *Server) handleGetPluginBadges(w http.ResponseWriter, r *http.Request) {
	if s.pluginManager == nil {
		respondJSON(w, http.StatusOK, []interface{}{})
		return
	}

	// Get container ID and host ID from query params
	hostIDStr := r.URL.Query().Get("host_id")
	containerID := r.URL.Query().Get("container_id")

	if hostIDStr == "" || containerID == "" {
		respondError(w, http.StatusBadRequest, "Missing host_id or container_id query parameter")
		return
	}

	hostID, err := strconv.ParseInt(hostIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host_id")
		return
	}

	// Get the container from storage
	containers, err := s.db.GetLatestContainers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var targetContainer *models.Container
	for i := range containers {
		if containers[i].HostID == hostID && containers[i].ID == containerID {
			targetContainer = &containers[i]
			break
		}
	}

	if targetContainer == nil {
		respondJSON(w, http.StatusOK, []interface{}{})
		return
	}

	badges := s.pluginManager.GetBadgesForContainer(r.Context(), *targetContainer)
	respondJSON(w, http.StatusOK, badges)
}

// handleEnablePlugin enables a plugin
func (s *Server) handleEnablePlugin(w http.ResponseWriter, r *http.Request) {
	if s.pluginManager == nil {
		respondError(w, http.StatusServiceUnavailable, "Plugin system not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.pluginManager.EnablePlugin(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Plugin enabled",
	})
}

// handleDisablePlugin disables a plugin
func (s *Server) handleDisablePlugin(w http.ResponseWriter, r *http.Request) {
	if s.pluginManager == nil {
		respondError(w, http.StatusServiceUnavailable, "Plugin system not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.pluginManager.DisablePlugin(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Plugin disabled",
	})
}

// handleGetPluginSettings returns settings for a plugin
func (s *Server) handleGetPluginSettings(w http.ResponseWriter, r *http.Request) {
	if s.pluginManager == nil {
		respondError(w, http.StatusServiceUnavailable, "Plugin system not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	// Get plugin to verify it exists
	plugin, ok := s.pluginManager.GetPlugin(id)
	if !ok {
		respondError(w, http.StatusNotFound, "Plugin not found")
		return
	}

	// Get settings from database
	settings, err := s.db.GetAllPluginSettings(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get schema for defaults
	schema := plugin.Settings()

	// Merge defaults with stored values
	result := make(map[string]string)
	if schema != nil {
		for _, field := range schema.Fields {
			if val, exists := settings[field.Key]; exists {
				result[field.Key] = val
			} else {
				result[field.Key] = field.Default
			}
		}
	} else {
		result = settings
	}

	respondJSON(w, http.StatusOK, result)
}

// handleUpdatePluginSettings updates settings for a plugin
func (s *Server) handleUpdatePluginSettings(w http.ResponseWriter, r *http.Request) {
	if s.pluginManager == nil {
		respondError(w, http.StatusServiceUnavailable, "Plugin system not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	// Get plugin to verify it exists
	_, ok := s.pluginManager.GetPlugin(id)
	if !ok {
		respondError(w, http.StatusNotFound, "Plugin not found")
		return
	}

	var settings map[string]string
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Save each setting
	for key, value := range settings {
		if err := s.db.SetPluginSetting(id, key, value); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Settings updated",
	})
}
