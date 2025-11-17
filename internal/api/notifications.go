package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/gorilla/mux"
)

// Notification Channel Handlers

func (s *Server) handleGetNotificationChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.db.GetNotificationChannels()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get notification channels: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, channels)
}

func (s *Server) handleCreateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	var channel models.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate channel type
	if channel.Type != models.ChannelTypeWebhook && channel.Type != models.ChannelTypeNtfy && channel.Type != models.ChannelTypeInApp {
		respondError(w, http.StatusBadRequest, "Invalid channel type. Must be: webhook, ntfy, or in_app")
		return
	}

	// Validate required fields based on type
	if channel.Type == models.ChannelTypeWebhook {
		if url, ok := channel.Config["url"].(string); !ok || url == "" {
			respondError(w, http.StatusBadRequest, "Webhook channel requires 'url' in config")
			return
		}
	} else if channel.Type == models.ChannelTypeNtfy {
		if topic, ok := channel.Config["topic"].(string); !ok || topic == "" {
			respondError(w, http.StatusBadRequest, "Ntfy channel requires 'topic' in config")
			return
		}
	}

	if err := s.db.SaveNotificationChannel(&channel); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create notification channel: "+err.Error())
		return
	}

	// Refresh channels in notification service
	if s.notificationService != nil {
		s.notificationService.RefreshChannels()
	}

	respondJSON(w, http.StatusCreated, channel)
}

func (s *Server) handleUpdateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	var channel models.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	channel.ID = id

	if err := s.db.SaveNotificationChannel(&channel); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update notification channel: "+err.Error())
		return
	}

	// Refresh channels in notification service
	if s.notificationService != nil {
		s.notificationService.RefreshChannels()
	}

	respondJSON(w, http.StatusOK, channel)
}

func (s *Server) handleDeleteNotificationChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	if err := s.db.DeleteNotificationChannel(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete notification channel: "+err.Error())
		return
	}

	// Refresh channels in notification service
	if s.notificationService != nil {
		s.notificationService.RefreshChannels()
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Channel deleted successfully"})
}

func (s *Server) handleTestNotificationChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	// Get the channel
	channel, err := s.db.GetNotificationChannel(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Channel not found")
		return
	}

	// Try to send test notification
	if s.notificationService != nil {
		// Send test notification
		ctx := r.Context()
		if err := s.notificationService.SendTestNotification(ctx, id); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to send test notification: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"message": "Test notification sent successfully to channel: " + channel.Name,
			"status":  "success",
		})
	} else {
		respondError(w, http.StatusServiceUnavailable, "Notification service not available")
	}
}

// Notification Rule Handlers

func (s *Server) handleGetNotificationRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.db.GetNotificationRules(false)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get notification rules: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, rules)
}

func (s *Server) handleCreateNotificationRule(w http.ResponseWriter, r *http.Request) {
	var rule models.NotificationRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate event types
	validEventTypes := map[string]bool{
		models.EventTypeNewImage:              true,
		models.EventTypeImageUpdateAvailable:  true,
		models.EventTypeStateChange:           true,
		models.EventTypeHighCPU:               true,
		models.EventTypeHighMemory:            true,
		models.EventTypeAnomalousBehavior:     true,
		models.EventTypeContainerStarted:      true,
		models.EventTypeContainerStopped:      true,
		models.EventTypeContainerPaused:       true,
		models.EventTypeContainerResumed:      true,
	}

	for _, et := range rule.EventTypes {
		if !validEventTypes[et] {
			respondError(w, http.StatusBadRequest, "Invalid event type: "+et)
			return
		}
	}

	// Set defaults if not provided
	if rule.ThresholdDurationSeconds == 0 {
		rule.ThresholdDurationSeconds = 120
	}
	if rule.CooldownSeconds == 0 {
		rule.CooldownSeconds = 300
	}

	if err := s.db.SaveNotificationRule(&rule); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create notification rule: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleUpdateNotificationRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid rule ID")
		return
	}

	var rule models.NotificationRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	rule.ID = id

	if err := s.db.SaveNotificationRule(&rule); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update notification rule: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, rule)
}

func (s *Server) handleDeleteNotificationRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid rule ID")
		return
	}

	if err := s.db.DeleteNotificationRule(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete notification rule: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Rule deleted successfully"})
}

// Notification Log Handlers

func (s *Server) handleGetNotificationLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	unreadOnlyStr := r.URL.Query().Get("unread")
	unreadOnly := unreadOnlyStr == "true"

	logs, err := s.db.GetNotificationLogs(limit, unreadOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get notification logs: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, logs)
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	if err := s.db.MarkNotificationRead(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to mark notification as read: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Notification marked as read"})
}

func (s *Server) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if err := s.db.MarkAllNotificationsRead(); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to mark all notifications as read: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "All notifications marked as read"})
}

func (s *Server) handleClearNotifications(w http.ResponseWriter, r *http.Request) {
	if err := s.db.ClearAllNotifications(); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to clear notifications: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "All notifications cleared"})
}

// Notification Silence Handlers

func (s *Server) handleGetNotificationSilences(w http.ResponseWriter, r *http.Request) {
	silences, err := s.db.GetActiveSilences()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get notification silences: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, silences)
}

func (s *Server) handleCreateNotificationSilence(w http.ResponseWriter, r *http.Request) {
	// Use a custom struct to handle flexible datetime formats from HTML inputs
	var req struct {
		HostID           *int64 `json:"host_id,omitempty"`
		ContainerID      string `json:"container_id,omitempty"`
		ContainerName    string `json:"container_name,omitempty"`
		HostPattern      string `json:"host_pattern,omitempty"`
		ContainerPattern string `json:"container_pattern,omitempty"`
		SilencedUntil    string `json:"silenced_until"`
		Reason           string `json:"reason,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Parse the datetime with flexible format support
	// HTML datetime-local inputs send: "2026-11-04T14:06" (without seconds/timezone)
	var silencedUntil time.Time
	var err error

	// Try multiple datetime formats
	formats := []string{
		time.RFC3339,                // "2006-01-02T15:04:05Z07:00"
		"2006-01-02T15:04:05",       // "2026-11-04T14:06:05"
		"2006-01-02T15:04",          // "2026-11-04T14:06" (HTML datetime-local)
		time.RFC3339Nano,            // with nanoseconds
	}

	for _, format := range formats {
		silencedUntil, err = time.Parse(format, req.SilencedUntil)
		if err == nil {
			break
		}
	}

	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid silenced_until format. Use ISO 8601 format (e.g., 2026-11-04T14:06)")
		return
	}

	silence := models.NotificationSilence{
		HostID:           req.HostID,
		ContainerID:      req.ContainerID,
		ContainerName:    req.ContainerName,
		HostPattern:      req.HostPattern,
		ContainerPattern: req.ContainerPattern,
		SilencedUntil:    silencedUntil,
		Reason:           req.Reason,
	}

	// Validate that silence has either host_id, container_id, or patterns
	if silence.HostID == nil && silence.ContainerID == "" && silence.HostPattern == "" && silence.ContainerPattern == "" {
		respondError(w, http.StatusBadRequest, "Silence must specify host_id, container_id, or a pattern")
		return
	}

	if err := s.db.SaveNotificationSilence(&silence); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create notification silence: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, silence)
}

func (s *Server) handleDeleteNotificationSilence(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid silence ID")
		return
	}

	if err := s.db.DeleteNotificationSilence(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete notification silence: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Silence deleted successfully"})
}

// Notification Status Handler

func (s *Server) handleGetNotificationStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.db.GetNotificationStatus()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get notification status: "+err.Error())
		return
	}

	// Add rate limiter info if available
	if s.notificationService != nil && s.notificationService.GetRateLimiter() != nil {
		rl := s.notificationService.GetRateLimiter()
		status.RateLimitRemaining = rl.GetRemaining()
		status.RateLimitReset = rl.GetResetTime()
	}

	respondJSON(w, http.StatusOK, status)
}
