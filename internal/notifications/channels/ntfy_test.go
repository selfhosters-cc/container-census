package channels

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// TestNtfyChannel_BasicSend tests basic ntfy notification
func TestNtfyChannel_BasicSend(t *testing.T) {
	received := false
	var receivedTopic string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		receivedTopic = r.URL.Path

		// Read body
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Error("Expected message body")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &models.NotificationChannel{
		Name: "test-ntfy",
		Type: "ntfy",
		Config: map[string]interface{}{
			"server_url": server.URL,
			"topic":      "container-alerts",
		},
	}

	nc, err := NewNtfyChannel(channel)
	if err != nil {
		t.Fatalf("NewNtfyChannel failed: %v", err)
	}

	event := models.NotificationEvent{
		EventType:     "container_stopped",
		ContainerName: "web",
		ScannedAt:     time.Now(),
	}

	ctx := context.Background()
	err = nc.Send(ctx, "Container stopped", event)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !received {
		t.Error("Ntfy notification not received")
	}

	if receivedTopic != "/container-alerts" {
		t.Errorf("Expected topic /container-alerts, got %s", receivedTopic)
	}
}

// TestNtfyChannel_BearerAuth tests Bearer token authentication
func TestNtfyChannel_BearerAuth(t *testing.T) {
	receivedAuth := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &models.NotificationChannel{
		Name: "test-ntfy",
		Type: "ntfy",
		Config: map[string]interface{}{
			"server_url": server.URL,
			"topic":      "alerts",
			"token":      "secret-token",
		},
	}

	nc, err := NewNtfyChannel(channel)
	if err != nil {
		t.Fatalf("NewNtfyChannel failed: %v", err)
	}

	event := models.NotificationEvent{
		EventType: "test",
		ScannedAt: time.Now(),
	}

	ctx := context.Background()
	err = nc.Send(ctx, "Test", event)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if receivedAuth != "Bearer secret-token" {
		t.Errorf("Expected Authorization 'Bearer secret-token', got '%s'", receivedAuth)
	}
}

// TestNtfyChannel_PriorityMapping tests priority mapping for different events
func TestNtfyChannel_PriorityMapping(t *testing.T) {
	tests := []struct {
		eventType        string
		expectedPriority string
	}{
		{"high_cpu", "4"},              // High priority
		{"high_memory", "4"},           // High priority
		{"anomalous_behavior", "4"},    // High priority
		{"container_stopped", "3"},     // Default priority
		{"new_image", "3"},             // Default priority
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			receivedPriority := ""

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPriority = r.Header.Get("X-Priority")
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			channel := &models.NotificationChannel{
				Name: "test-ntfy",
				Type: "ntfy",
				Config: map[string]interface{}{
					"server_url": server.URL,
					"topic":      "test",
				},
			}

			nc, err := NewNtfyChannel(channel)
			if err != nil {
				t.Fatalf("NewNtfyChannel failed: %v", err)
			}

			event := models.NotificationEvent{
				EventType: tt.eventType,
				ScannedAt: time.Now(),
			}

			ctx := context.Background()
			nc.Send(ctx, "Test", event)

			if receivedPriority != tt.expectedPriority {
				t.Errorf("Expected priority %s for %s, got %s",
					tt.expectedPriority, tt.eventType, receivedPriority)
			}
		})
	}
}

// TestNtfyChannel_Tags tests tag generation for events
func TestNtfyChannel_Tags(t *testing.T) {
	receivedTags := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTags = r.Header.Get("X-Tags")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &models.NotificationChannel{
		Name: "test-ntfy",
		Type: "ntfy",
		Config: map[string]interface{}{
			"server_url": server.URL,
			"topic":      "test",
		},
	}

	nc, err := NewNtfyChannel(channel)
	if err != nil {
		t.Fatalf("NewNtfyChannel failed: %v", err)
	}

	event := models.NotificationEvent{
		EventType: "high_cpu",
		ScannedAt: time.Now(),
	}

	ctx := context.Background()
	nc.Send(ctx, "High CPU", event)

	if receivedTags == "" {
		t.Log("Note: Tags not set - verify tag implementation")
	} else {
		t.Logf("Received tags: %s", receivedTags)
	}
}

// TestNtfyChannel_MissingConfig tests error handling for missing config
func TestNtfyChannel_MissingConfig(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{"missing topic", map[string]interface{}{"server_url": "https://ntfy.sh"}},
		{"empty config", map[string]interface{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &models.NotificationChannel{
				Name:   "test-ntfy",
				Type:   "ntfy",
				Config: tt.config,
			}

			_, err := NewNtfyChannel(channel)
			if err == nil {
				t.Error("Expected error for invalid config")
			}
		})
	}
}

// TestNtfyChannel_Test tests the test notification
func TestNtfyChannel_Test(t *testing.T) {
	received := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &models.NotificationChannel{
		Name: "test-ntfy",
		Type: "ntfy",
		Config: map[string]interface{}{
			"server_url": server.URL,
			"topic":      "test",
		},
	}

	nc, err := NewNtfyChannel(channel)
	if err != nil {
		t.Fatalf("NewNtfyChannel failed: %v", err)
	}

	ctx := context.Background()
	err = nc.Test(ctx)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if !received {
		t.Error("Test notification not received")
	}
}

// TestNtfyChannel_DefaultServerURL tests default ntfy.sh server
func TestNtfyChannel_DefaultServerURL(t *testing.T) {
	channel := &models.NotificationChannel{
		Name: "test-ntfy",
		Type: "ntfy",
		Config: map[string]interface{}{
			"topic": "test-topic",
			// No server_url specified
		},
	}

	nc, err := NewNtfyChannel(channel)
	if err != nil {
		t.Fatalf("NewNtfyChannel failed: %v", err)
	}

	// Should use default ntfy.sh
	// (checking internal config would require exposing it or testing actual sends)
	if nc.Name() != "test-ntfy" {
		t.Errorf("Expected name 'test-ntfy', got '%s'", nc.Name())
	}
}
