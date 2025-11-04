package storage

import (
	"testing"

	"github.com/container-census/container-census/internal/models"
)

// TestInitializeDefaultRules tests that default notification rules are created
func TestInitializeDefaultRules(t *testing.T) {
	db := setupTestDB(t)

	// Initialize default rules
	err := db.InitializeDefaultNotifications()
	if err != nil {
		t.Fatalf("InitializeDefaultNotifications failed: %v", err)
	}

	// Get all rules
	rules, err := db.GetNotificationRules(false)
	if err != nil {
		t.Fatalf("GetNotificationRules failed: %v", err)
	}

	// Should have created default rules
	if len(rules) == 0 {
		t.Fatal("Expected default rules to be created")
	}

	// Check for expected default rules
	ruleNames := make(map[string]bool)
	for _, rule := range rules {
		ruleNames[rule.Name] = true
	}

	expectedRules := []string{
		"Container Stopped",
		"New Image Detected",
		"High Resource Usage",
	}

	for _, name := range expectedRules {
		if !ruleNames[name] {
			t.Errorf("Expected default rule '%s' not found", name)
		}
	}

	// Verify channels were created
	channels, err := db.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels failed: %v", err)
	}

	if len(channels) == 0 {
		t.Fatal("Expected default in-app channel to be created")
	}

	// Should have an in-app channel
	hasInApp := false
	for _, ch := range channels {
		if ch.Type == "inapp" {
			hasInApp = true
			break
		}
	}

	if !hasInApp {
		t.Error("Expected default in-app channel")
	}

	// Verify rules are linked to channels
	for _, rule := range rules {
		if len(rule.ChannelIDs) == 0 {
			t.Errorf("Rule '%s' should be linked to channels", rule.Name)
		}
	}
}

// TestInitializeDefaultRulesIdempotent tests that running initialization twice doesn't duplicate
func TestInitializeDefaultRulesIdempotent(t *testing.T) {
	db := setupTestDB(t)

	// Run initialization twice
	err := db.InitializeDefaultNotifications()
	if err != nil {
		t.Fatalf("First InitializeDefaultNotifications failed: %v", err)
	}

	err = db.InitializeDefaultNotifications()
	if err != nil {
		t.Fatalf("Second InitializeDefaultNotifications failed: %v", err)
	}

	// Get all rules
	rules, err := db.GetNotificationRules(false)
	if err != nil {
		t.Fatalf("GetNotificationRules failed: %v", err)
	}

	// Count each rule name
	ruleCounts := make(map[string]int)
	for _, rule := range rules {
		ruleCounts[rule.Name]++
	}

	// Verify no duplicates
	for name, count := range ruleCounts {
		if count > 1 {
			t.Errorf("Rule '%s' appears %d times (should be 1)", name, count)
		}
	}

	// Verify channels aren't duplicated
	channels, err := db.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels failed: %v", err)
	}

	channelCounts := make(map[string]int)
	for _, ch := range channels {
		channelCounts[ch.Name]++
	}

	for name, count := range channelCounts {
		if count > 1 {
			t.Errorf("Channel '%s' appears %d times (should be 1)", name, count)
		}
	}
}

// TestDefaultRuleConfiguration tests the configuration of default rules
func TestDefaultRuleConfiguration(t *testing.T) {
	db := setupTestDB(t)

	err := db.InitializeDefaultNotifications()
	if err != nil {
		t.Fatalf("InitializeDefaultNotifications failed: %v", err)
	}

	rules, err := db.GetNotificationRules(false)
	if err != nil {
		t.Fatalf("GetNotificationRules failed: %v", err)
	}

	// Check "Container Stopped" rule
	for _, rule := range rules {
		if rule.Name == "Container Stopped" {
			if len(rule.EventTypes) == 0 {
				t.Error("Container Stopped rule should have event types")
			}

			hasStoppedEvent := false
			for _, et := range rule.EventTypes {
				if et == "container_stopped" {
					hasStoppedEvent = true
					break
				}
			}

			if !hasStoppedEvent {
				t.Error("Container Stopped rule should include 'container_stopped' event")
			}

			if !rule.Enabled {
				t.Error("Default rules should be enabled")
			}
		}

		if rule.Name == "High Resource Usage" {
			if rule.CPUThreshold == nil && rule.MemoryThreshold == nil {
				t.Error("High Resource Usage rule should have thresholds configured")
			}

			if rule.ThresholdDurationSeconds <= 0 {
				t.Error("High Resource Usage rule should have threshold duration")
			}

			if rule.CooldownSeconds <= 0 {
				t.Error("High Resource Usage rule should have cooldown period")
			}
		}

		if rule.Name == "New Image Detected" {
			hasImageEvent := false
			for _, et := range rule.EventTypes {
				if et == "new_image" {
					hasImageEvent = true
					break
				}
			}

			if !hasImageEvent {
				t.Error("New Image Detected rule should include 'new_image' event")
			}
		}
	}
}

// TestDefaultRulesWithExistingData tests initialization when data already exists
func TestDefaultRulesWithExistingData(t *testing.T) {
	db := setupTestDB(t)

	// Create a custom channel first
	channel := &models.NotificationChannel{
		Name:    "custom-channel",
		Type:    "webhook",
		Config:  map[string]interface{}{"url": "https://example.com"},
		Enabled: true,
	}
	if err := db.SaveNotificationChannel(channel); err != nil {
		t.Fatalf("Failed to save custom channel: %v", err)
	}

	// Create a custom rule
	rule := &models.NotificationRule{
		Name:       "custom-rule",
		EventTypes: []string{"container_started"},
		Enabled:    true,
		ChannelIDs: []int64{channel.ID},
	}
	if err := db.SaveNotificationRule(rule); err != nil {
		t.Fatalf("Failed to save custom rule: %v", err)
	}

	// Now initialize defaults
	err := db.InitializeDefaultNotifications()
	if err != nil {
		t.Fatalf("InitializeDefaultNotifications failed: %v", err)
	}

	// Get all rules
	rules, err := db.GetNotificationRules(false)
	if err != nil {
		t.Fatalf("GetNotificationRules failed: %v", err)
	}

	// Should have both custom and default rules
	hasCustom := false
	hasDefault := false

	for _, r := range rules {
		if r.Name == "custom-rule" {
			hasCustom = true
		}
		if r.Name == "Container Stopped" {
			hasDefault = true
		}
	}

	if !hasCustom {
		t.Error("Custom rule should be preserved")
	}

	if !hasDefault {
		t.Error("Default rules should be created")
	}

	// Verify channels
	channels, err := db.GetNotificationChannels()
	if err != nil {
		t.Fatalf("GetNotificationChannels failed: %v", err)
	}

	hasCustomChannel := false
	hasInAppChannel := false

	for _, ch := range channels {
		if ch.Name == "custom-channel" {
			hasCustomChannel = true
		}
		if ch.Type == "inapp" {
			hasInAppChannel = true
		}
	}

	if !hasCustomChannel {
		t.Error("Custom channel should be preserved")
	}

	if !hasInAppChannel {
		t.Error("Default in-app channel should be created")
	}
}
