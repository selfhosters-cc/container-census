package storage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// TestNotificationChannelCRUD tests Create, Read, Update, Delete for notification channels
func TestNotificationChannelCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create a webhook channel
	config := map[string]interface{}{
		"url": "https://example.com/webhook",
		"headers": map[string]string{
			"Authorization": "Bearer token123",
		},
	}
	configJSON, _ := json.Marshal(config)

	channel := &models.NotificationChannel{
		Name:    "test-webhook",
		Type:    "webhook",
		Config:  string(configJSON),
		Enabled: true,
	}

	err := db.SaveNotificationChannel(channel)
	if err != nil {
		t.Fatalf("SaveNotificationChannel failed: %v", err)
	}

	if channel.ID == 0 {
		t.Error("Expected channel ID to be set after save")
	}

	// Read channels
	channels, err := db.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels failed: %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("Expected 1 channel, got %d", len(channels))
	}

	savedChannel := channels[0]
	if savedChannel.Name != channel.Name {
		t.Errorf("Expected name %s, got %s", channel.Name, savedChannel.Name)
	}
	if savedChannel.Type != channel.Type {
		t.Errorf("Expected type %s, got %s", channel.Type, savedChannel.Type)
	}
	if !savedChannel.Enabled {
		t.Error("Expected channel to be enabled")
	}

	// Update channel
	savedChannel.Name = "updated-webhook"
	savedChannel.Enabled = false

	err = db.SaveNotificationChannel(savedChannel)
	if err != nil {
		t.Fatalf("SaveNotificationChannel (update) failed: %v", err)
	}

	// Verify update
	channels, err = db.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels failed: %v", err)
	}

	if channels[0].Name != "updated-webhook" {
		t.Error("Channel name not updated")
	}
	if channels[0].Enabled {
		t.Error("Channel should be disabled")
	}

	// Delete channel
	err = db.DeleteNotificationChannel(savedChannel.ID)
	if err != nil {
		t.Fatalf("DeleteNotificationChannel failed: %v", err)
	}

	// Verify deletion
	channels, err = db.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels failed: %v", err)
	}

	if len(channels) != 0 {
		t.Errorf("Expected 0 channels after deletion, got %d", len(channels))
	}
}

// TestMultipleChannelTypes tests different channel types
func TestMultipleChannelTypes(t *testing.T) {
	db := setupTestDB(t)

	channels := []*models.NotificationChannel{
		{Name: "webhook1", Type: "webhook", Config: `{"url":"https://example.com"}`, Enabled: true},
		{Name: "ntfy1", Type: "ntfy", Config: `{"server_url":"https://ntfy.sh","topic":"alerts"}`, Enabled: true},
		{Name: "inapp1", Type: "inapp", Config: `{}`, Enabled: true},
	}

	for _, ch := range channels {
		if err := db.SaveNotificationChannel(ch); err != nil {
			t.Fatalf("Failed to save channel %s: %v", ch.Name, err)
		}
	}

	retrieved, err := db.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Fatalf("Expected 3 channels, got %d", len(retrieved))
	}

	// Verify types
	types := make(map[string]bool)
	for _, ch := range retrieved {
		types[ch.Type] = true
	}

	for _, expected := range []string{"webhook", "ntfy", "inapp"} {
		if !types[expected] {
			t.Errorf("Expected channel type %s not found", expected)
		}
	}
}

// TestNotificationRuleCRUD tests Create, Read, Update, Delete for notification rules
func TestNotificationRuleCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create channels first
	channel := &models.NotificationChannel{
		Name:    "test-channel",
		Type:    "inapp",
		Config:  `{}`,
		Enabled: true,
	}
	if err := db.SaveNotificationChannel(channel); err != nil {
		t.Fatalf("Failed to save channel: %v", err)
	}

	// Create a rule
	rule := &models.NotificationRule{
		Name:              "test-rule",
		EventTypes:        []string{"container_stopped", "new_image"},
		ContainerPattern:  "web-*",
		ImagePattern:      "nginx:*",
		CPUThreshold:      80.0,
		MemoryThreshold:   90.0,
		ThresholdDuration: 120,
		CooldownPeriod:    300,
		Enabled:           true,
		ChannelIDs:        []int{channel.ID},
	}

	err := db.SaveNotificationRule(rule)
	if err != nil {
		t.Fatalf("SaveNotificationRule failed: %v", err)
	}

	if rule.ID == 0 {
		t.Error("Expected rule ID to be set after save")
	}

	// Read rules
	rules, err := db.GetNotificationRules()
	if err != nil {
		t.Fatalf("GetNotificationRules failed: %v", err)
	}

	if len(rules) == 0 {
		t.Fatal("Expected at least 1 rule (including defaults)")
	}

	// Find our rule
	var savedRule *models.NotificationRule
	for i := range rules {
		if rules[i].Name == "test-rule" {
			savedRule = &rules[i]
			break
		}
	}

	if savedRule == nil {
		t.Fatal("Created rule not found")
	}

	if savedRule.ContainerPattern != rule.ContainerPattern {
		t.Errorf("Expected container pattern %s, got %s", rule.ContainerPattern, savedRule.ContainerPattern)
	}
	if savedRule.CPUThreshold != rule.CPUThreshold {
		t.Errorf("Expected CPU threshold %f, got %f", rule.CPUThreshold, savedRule.CPUThreshold)
	}
	if len(savedRule.EventTypes) != 2 {
		t.Errorf("Expected 2 event types, got %d", len(savedRule.EventTypes))
	}
	if len(savedRule.ChannelIDs) != 1 {
		t.Errorf("Expected 1 channel, got %d", len(savedRule.ChannelIDs))
	}

	// Update rule
	savedRule.Name = "updated-rule"
	savedRule.ContainerPattern = "api-*"
	savedRule.Enabled = false

	err = db.SaveNotificationRule(savedRule)
	if err != nil {
		t.Fatalf("SaveNotificationRule (update) failed: %v", err)
	}

	// Verify update
	rules, err = db.GetNotificationRules()
	if err != nil {
		t.Fatalf("GetNotificationRules failed: %v", err)
	}

	var updatedRule *models.NotificationRule
	for i := range rules {
		if rules[i].ID == savedRule.ID {
			updatedRule = &rules[i]
			break
		}
	}

	if updatedRule.Name != "updated-rule" {
		t.Error("Rule name not updated")
	}
	if updatedRule.Enabled {
		t.Error("Rule should be disabled")
	}

	// Delete rule
	err = db.DeleteNotificationRule(savedRule.ID)
	if err != nil {
		t.Fatalf("DeleteNotificationRule failed: %v", err)
	}

	// Verify deletion
	rules, err = db.GetNotificationRules()
	if err != nil {
		t.Fatalf("GetNotificationRules failed: %v", err)
	}

	for _, r := range rules {
		if r.ID == savedRule.ID {
			t.Error("Rule should be deleted")
		}
	}
}

// TestNotificationRuleChannelMapping tests many-to-many relationship
func TestNotificationRuleChannelMapping(t *testing.T) {
	db := setupTestDB(t)

	// Create multiple channels
	channels := []*models.NotificationChannel{
		{Name: "channel1", Type: "inapp", Config: `{}`, Enabled: true},
		{Name: "channel2", Type: "webhook", Config: `{"url":"https://example.com"}`, Enabled: true},
		{Name: "channel3", Type: "ntfy", Config: `{"topic":"test"}`, Enabled: true},
	}

	for _, ch := range channels {
		if err := db.SaveNotificationChannel(ch); err != nil {
			t.Fatalf("Failed to save channel: %v", err)
		}
	}

	// Create rule with multiple channels
	rule := &models.NotificationRule{
		Name:       "multi-channel-rule",
		EventTypes: []string{"container_stopped"},
		Enabled:    true,
		ChannelIDs: []int{channels[0].ID, channels[1].ID, channels[2].ID},
	}

	if err := db.SaveNotificationRule(rule); err != nil {
		t.Fatalf("SaveNotificationRule failed: %v", err)
	}

	// Retrieve and verify
	rules, err := db.GetNotificationRules()
	if err != nil {
		t.Fatalf("GetNotificationRules failed: %v", err)
	}

	var found *models.NotificationRule
	for i := range rules {
		if rules[i].ID == rule.ID {
			found = &rules[i]
			break
		}
	}

	if found == nil {
		t.Fatal("Rule not found")
	}

	if len(found.ChannelIDs) != 3 {
		t.Errorf("Expected 3 channels, got %d", len(found.ChannelIDs))
	}

	// Update rule to remove one channel
	found.ChannelIDs = []int{channels[0].ID, channels[2].ID}
	if err := db.SaveNotificationRule(found); err != nil {
		t.Fatalf("Failed to update rule: %v", err)
	}

	// Verify update
	rules, err = db.GetNotificationRules()
	if err != nil {
		t.Fatalf("GetNotificationRules failed: %v", err)
	}

	for _, r := range rules {
		if r.ID == rule.ID {
			if len(r.ChannelIDs) != 2 {
				t.Errorf("Expected 2 channels after update, got %d", len(r.ChannelIDs))
			}
		}
	}
}

// TestNotificationLog tests notification log operations
func TestNotificationLog(t *testing.T) {
	db := setupTestDB(t)

	// Create host first
	host := &models.Host{Name: "log-host", Address: "unix:///", Enabled: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	// Save notification logs
	now := time.Now()
	logs := []models.NotificationLog{
		{
			RuleName:      "rule1",
			EventType:     "container_stopped",
			ContainerName: "web-1",
			HostID:        host.ID,
			Message:       "Container web-1 stopped",
			ScannedAt:     now.Add(-5 * time.Minute),
			Read:          false,
		},
		{
			RuleName:      "rule2",
			EventType:     "new_image",
			ContainerName: "api-1",
			HostID:        host.ID,
			Message:       "New image detected",
			ScannedAt:     now.Add(-2 * time.Minute),
			Read:          false,
		},
	}

	for _, log := range logs {
		if err := db.SaveNotificationLog(log); err != nil {
			t.Fatalf("SaveNotificationLog failed: %v", err)
		}
	}

	// Get all logs
	retrieved, err := db.GetNotificationLogs(100, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}

	if len(retrieved) < 2 {
		t.Fatalf("Expected at least 2 logs, got %d", len(retrieved))
	}

	// Get unread logs only
	unread, err := db.GetNotificationLogs(100, true)
	if err != nil {
		t.Fatalf("GetNotificationLogs (unread) failed: %v", err)
	}

	if len(unread) < 2 {
		t.Errorf("Expected at least 2 unread logs, got %d", len(unread))
	}

	// Mark one as read
	if len(retrieved) > 0 {
		err = db.MarkNotificationRead(retrieved[0].ID)
		if err != nil {
			t.Fatalf("MarkNotificationRead failed: %v", err)
		}

		// Verify it's marked read
		unread, err = db.GetNotificationLogs(100, true)
		if err != nil {
			t.Fatalf("GetNotificationLogs failed: %v", err)
		}

		// Should have one less unread
		found := false
		for _, log := range unread {
			if log.ID == retrieved[0].ID {
				found = true
				break
			}
		}

		if found {
			t.Error("Marked log should not appear in unread list")
		}
	}

	// Mark all as read
	err = db.MarkAllNotificationsRead()
	if err != nil {
		t.Fatalf("MarkAllNotificationsRead failed: %v", err)
	}

	unread, err = db.GetNotificationLogs(100, true)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}

	if len(unread) != 0 {
		t.Errorf("Expected 0 unread logs after mark all read, got %d", len(unread))
	}
}

// TestNotificationLogClear tests clearing old notifications
// NOTE: User indicated this might not be working correctly
func TestNotificationLogClear(t *testing.T) {
	db := setupTestDB(t)

	// Create host
	host := &models.Host{Name: "clear-host", Address: "unix:///", Enabled: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create old logs (8 days old)
	for i := 0; i < 5; i++ {
		log := models.NotificationLog{
			RuleName:      "old-rule",
			EventType:     "container_stopped",
			ContainerName: "old-container",
			HostID:        host.ID,
			Message:       "Old notification",
			ScannedAt:     now.Add(-8 * 24 * time.Hour),
			Read:          true,
		}
		if err := db.SaveNotificationLog(log); err != nil {
			t.Fatalf("Failed to save old log: %v", err)
		}
	}

	// Create recent logs
	for i := 0; i < 3; i++ {
		log := models.NotificationLog{
			RuleName:      "new-rule",
			EventType:     "new_image",
			ContainerName: "new-container",
			HostID:        host.ID,
			Message:       "Recent notification",
			ScannedAt:     now.Add(-1 * time.Hour),
			Read:          false,
		}
		if err := db.SaveNotificationLog(log); err != nil {
			t.Fatalf("Failed to save recent log: %v", err)
		}
	}

	// Get initial count
	beforeLogs, err := db.GetNotificationLogs(1000, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}
	beforeCount := len(beforeLogs)
	t.Logf("Logs before clear: %d", beforeCount)

	// Clear old logs
	err = db.CleanupOldNotifications()
	if err != nil {
		t.Fatalf("CleanupOldNotifications failed: %v", err)
	}

	// Get logs after clear
	afterLogs, err := db.GetNotificationLogs(1000, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}
	afterCount := len(afterLogs)

	t.Logf("Logs after clear: %d", afterCount)

	// Should have removed old logs but kept recent ones
	// Implementation should keep 100 most recent OR delete those older than 7 days
	if afterCount >= beforeCount {
		t.Errorf("Expected logs to be cleared, before: %d, after: %d", beforeCount, afterCount)
	}

	// Verify recent logs are still there
	foundRecent := false
	for _, log := range afterLogs {
		if log.RuleName == "new-rule" {
			foundRecent = true
			break
		}
	}

	if !foundRecent && afterCount > 0 {
		t.Error("Recent logs should be preserved")
	}
}

// TestNotificationSilences tests silence management
func TestNotificationSilences(t *testing.T) {
	db := setupTestDB(t)

	// Create host
	host := &models.Host{Name: "silence-host", Address: "unix:///", Enabled: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create silences
	silences := []models.NotificationSilence{
		{
			HostID:           &host.ID,
			ContainerPattern: "web-*",
			ExpiresAt:        now.Add(1 * time.Hour),
			Reason:           "Maintenance window",
		},
		{
			ID: "specific123",
			HostID:      &host.ID,
			ExpiresAt:   now.Add(2 * time.Hour),
			Reason:      "Known issue",
		},
		{
			// Expired silence
			HostID:           &host.ID,
			ContainerPattern: "old-*",
			ExpiresAt:        now.Add(-1 * time.Hour),
			Reason:           "Expired",
		},
	}

	for _, silence := range silences {
		if err := db.SaveNotificationSilence(silence); err != nil {
			t.Fatalf("SaveNotificationSilence failed: %v", err)
		}
	}

	// Get active silences (should not include expired)
	active, err := db.GetActiveSilences()
	if err != nil {
		t.Fatalf("GetActiveSilences failed: %v", err)
	}

	if len(active) != 2 {
		t.Errorf("Expected 2 active silences, got %d", len(active))
	}

	// Verify expired silence is not included
	for _, s := range active {
		if s.ContainerPattern == "old-*" {
			t.Error("Expired silence should not be in active list")
		}
	}

	// Delete a silence
	if len(active) > 0 {
		err = db.DeleteNotificationSilence(active[0].ID)
		if err != nil {
			t.Fatalf("DeleteNotificationSilence failed: %v", err)
		}

		// Verify deletion
		remaining, err := db.GetActiveSilences()
		if err != nil {
			t.Fatalf("GetActiveSilences failed: %v", err)
		}

		if len(remaining) != 1 {
			t.Errorf("Expected 1 silence after deletion, got %d", len(remaining))
		}
	}
}

// TestBaselineStats tests container baseline statistics
func TestBaselineStats(t *testing.T) {
	db := setupTestDB(t)

	// Create host
	host := &models.Host{Name: "baseline-host", Address: "unix:///", Enabled: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	baseline := models.ContainerBaselineStats{
		ID:    "baseline123",
		HostID:         host.ID,
		ImageID:        "sha256:abc123",
		AvgCPUPercent:  45.5,
		AvgMemoryUsage: 524288000,
		SampleCount:    20,
		CapturedAt:     now,
	}

	// Save baseline
	err := db.SaveContainerBaseline(baseline)
	if err != nil {
		t.Fatalf("SaveContainerBaseline failed: %v", err)
	}

	// Get baseline
	retrieved, err := db.GetContainerBaseline(host.ID, "baseline123")
	if err != nil {
		t.Fatalf("GetContainerBaseline failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected baseline to be retrieved")
	}

	if retrieved.AvgCPUPercent != baseline.AvgCPUPercent {
		t.Errorf("Expected avg CPU %f, got %f", baseline.AvgCPUPercent, retrieved.AvgCPUPercent)
	}
	if retrieved.SampleCount != baseline.SampleCount {
		t.Errorf("Expected sample count %d, got %d", baseline.SampleCount, retrieved.SampleCount)
	}

	// Update baseline (new image)
	baseline.ImageID = "sha256:def456"
	baseline.AvgCPUPercent = 50.0
	baseline.CapturedAt = now.Add(1 * time.Hour)

	err = db.SaveContainerBaseline(baseline)
	if err != nil {
		t.Fatalf("SaveContainerBaseline (update) failed: %v", err)
	}

	// Verify update
	retrieved, err = db.GetContainerBaseline(host.ID, "baseline123")
	if err != nil {
		t.Fatalf("GetContainerBaseline failed: %v", err)
	}

	if retrieved.AvgCPUPercent != 50.0 {
		t.Error("Baseline should be updated")
	}
}

// TestThresholdState tests notification threshold state tracking
func TestThresholdState(t *testing.T) {
	db := setupTestDB(t)

	// Create host
	host := &models.Host{Name: "threshold-host", Address: "unix:///", Enabled: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Save threshold state
	state := models.NotificationThresholdState{
		ID:  "threshold123",
		HostID:       host.ID,
		ThresholdType: "high_cpu",
		BreachStart:  now.Add(-5 * time.Minute),
		LastChecked:  now,
	}

	err := db.SaveThresholdState(state)
	if err != nil {
		t.Fatalf("SaveThresholdState failed: %v", err)
	}

	// Get threshold state
	retrieved, err := db.GetThresholdState(host.ID, "threshold123", "high_cpu")
	if err != nil {
		t.Fatalf("GetThresholdState failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected threshold state to be retrieved")
	}

	if !retrieved.BreachStart.Equal(state.BreachStart) {
		t.Error("Breach start time mismatch")
	}

	// Clear threshold state
	err = db.ClearThresholdState(host.ID, "threshold123", "high_cpu")
	if err != nil {
		t.Fatalf("ClearThresholdState failed: %v", err)
	}

	// Verify cleared
	retrieved, err = db.GetThresholdState(host.ID, "threshold123", "high_cpu")
	if err != nil {
		t.Fatalf("GetThresholdState failed: %v", err)
	}

	if retrieved != nil {
		t.Error("Threshold state should be cleared")
	}
}

// TestGetLastNotificationTime tests cooldown tracking
func TestGetLastNotificationTime(t *testing.T) {
	db := setupTestDB(t)

	// Create host
	host := &models.Host{Name: "cooldown-host", Address: "unix:///", Enabled: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Save a notification
	log := models.NotificationLog{
		RuleName:      "test-rule",
		EventType:     "container_stopped",
		ContainerName: "cooldown-container",
		ID:   "cooldown123",
		HostID:        host.ID,
		Message:       "Test notification",
		ScannedAt:     now.Add(-10 * time.Minute),
		Read:          false,
	}

	if err := db.SaveNotificationLog(log); err != nil {
		t.Fatalf("SaveNotificationLog failed: %v", err)
	}

	// Get last notification time
	lastTime, err := db.GetLastNotificationTime(host.ID, "cooldown123", "container_stopped")
	if err != nil {
		t.Fatalf("GetLastNotificationTime failed: %v", err)
	}

	if lastTime == nil {
		t.Fatal("Expected last notification time to be found")
	}

	// Should be approximately 10 minutes ago
	elapsed := now.Sub(*lastTime)
	if elapsed < 9*time.Minute || elapsed > 11*time.Minute {
		t.Errorf("Expected ~10 minutes elapsed, got %v", elapsed)
	}

	// Test non-existent container
	lastTime, err = db.GetLastNotificationTime(host.ID, "nonexistent", "container_stopped")
	if err != nil {
		t.Fatalf("GetLastNotificationTime failed: %v", err)
	}

	if lastTime != nil {
		t.Error("Expected no last notification time for non-existent container")
	}
}
