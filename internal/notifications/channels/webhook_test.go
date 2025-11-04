package channels

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// TestWebhookChannel_SuccessfulDelivery tests successful webhook delivery
func TestWebhookChannel_SuccessfulDelivery(t *testing.T) {
	// Create test server
	received := false
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true

		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Read payload
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook channel
	channel := &models.NotificationChannel{
		Name: "test-webhook",
		Type: "webhook",
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	wc, err := NewWebhookChannel(channel)
	if err != nil {
		t.Fatalf("NewWebhookChannel failed: %v", err)
	}

	// Send notification
	event := models.NotificationEvent{
		EventType:     "container_stopped",
		ContainerID:   "test123",
		ContainerName: "web-server",
		HostID:        1,
		HostName:      "host1",
		Image:         "nginx:latest",
		Timestamp:     time.Now(),
	}

	ctx := context.Background()
	err = wc.Send(ctx, "Container stopped", event)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !received {
		t.Error("Webhook was not received")
	}

	// Verify payload
	if receivedPayload["message"] != "Container stopped" {
		t.Errorf("Expected message 'Container stopped', got %v", receivedPayload["message"])
	}

	if receivedPayload["container_name"] != "web-server" {
		t.Errorf("Expected container_name 'web-server', got %v", receivedPayload["container_name"])
	}
}

// TestWebhookChannel_CustomHeaders tests custom headers
func TestWebhookChannel_CustomHeaders(t *testing.T) {
	receivedHeaders := make(http.Header)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture headers
		for key, values := range r.Header {
			receivedHeaders[key] = values
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &models.NotificationChannel{
		Name: "test-webhook",
		Type: "webhook",
		Config: map[string]interface{}{
			"url": server.URL,
			"headers": map[string]string{
				"Authorization": "Bearer secret-token",
				"X-Custom-Header": "custom-value",
			},
		},
	}

	wc, err := NewWebhookChannel(channel)
	if err != nil {
		t.Fatalf("NewWebhookChannel failed: %v", err)
	}

	event := models.NotificationEvent{
		EventType:     "test",
		ContainerName: "test",
		Timestamp:     time.Now(),
	}

	ctx := context.Background()
	err = wc.Send(ctx, "Test", event)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify custom headers
	if receivedHeaders.Get("Authorization") != "Bearer secret-token" {
		t.Errorf("Expected Authorization header, got %s", receivedHeaders.Get("Authorization"))
	}

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Expected X-Custom-Header, got %s", receivedHeaders.Get("X-Custom-Header"))
	}

	if receivedHeaders.Get("User-Agent") != "Container-Census-Notifier/1.0" {
		t.Errorf("Expected User-Agent header, got %s", receivedHeaders.Get("User-Agent"))
	}
}

// TestWebhookChannel_RetryLogic tests retry on failure
func TestWebhookChannel_RetryLogic(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++

		// Fail first 2 attempts, succeed on 3rd
		if attemptCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	channel := &models.NotificationChannel{
		Name: "test-webhook",
		Type: "webhook",
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	wc, err := NewWebhookChannel(channel)
	if err != nil {
		t.Fatalf("NewWebhookChannel failed: %v", err)
	}

	event := models.NotificationEvent{
		EventType:     "test",
		ContainerName: "test",
		Timestamp:     time.Now(),
	}

	ctx := context.Background()
	err = wc.Send(ctx, "Test", event)
	if err != nil {
		t.Fatalf("Send failed after retries: %v", err)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

// TestWebhookChannel_RetryExhaustion tests failure after 3 attempts
func TestWebhookChannel_RetryExhaustion(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	channel := &models.NotificationChannel{
		Name: "test-webhook",
		Type: "webhook",
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	wc, err := NewWebhookChannel(channel)
	if err != nil {
		t.Fatalf("NewWebhookChannel failed: %v", err)
	}

	event := models.NotificationEvent{
		EventType:     "test",
		ContainerName: "test",
		Timestamp:     time.Now(),
	}

	ctx := context.Background()
	err = wc.Send(ctx, "Test", event)
	if err == nil {
		t.Error("Expected error after retry exhaustion")
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

// TestWebhookChannel_AllEventFields tests that all event fields are included
func TestWebhookChannel_AllEventFields(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &models.NotificationChannel{
		Name: "test-webhook",
		Type: "webhook",
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	wc, err := NewWebhookChannel(channel)
	if err != nil {
		t.Fatalf("NewWebhookChannel failed: %v", err)
	}

	// Event with all optional fields
	event := models.NotificationEvent{
		EventType:     "new_image",
		ContainerID:   "abc123",
		ContainerName: "app",
		HostID:        1,
		HostName:      "host1",
		Image:         "app:v2",
		OldState:      "running",
		NewState:      "running",
		OldImage:      "app:v1",
		NewImage:      "app:v2",
		CPUPercent:    85.5,
		MemoryPercent: 92.3,
		Metadata: map[string]interface{}{
			"key": "value",
		},
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	err = wc.Send(ctx, "Image updated", event)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify all fields present
	expectedFields := []string{
		"message", "event_type", "timestamp", "container_id",
		"container_name", "host_id", "host_name", "image",
		"old_state", "new_state", "old_image", "new_image",
		"cpu_percent", "memory_percent", "metadata",
	}

	for _, field := range expectedFields {
		if _, exists := receivedPayload[field]; !exists {
			t.Errorf("Expected field %s not in payload", field)
		}
	}
}

// TestWebhookChannel_Test tests the test notification
func TestWebhookChannel_Test(t *testing.T) {
	received := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &models.NotificationChannel{
		Name: "test-webhook",
		Type: "webhook",
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	wc, err := NewWebhookChannel(channel)
	if err != nil {
		t.Fatalf("NewWebhookChannel failed: %v", err)
	}

	ctx := context.Background()
	err = wc.Test(ctx)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if !received {
		t.Error("Test notification was not received")
	}
}

// TestWebhookChannel_MissingURL tests error when URL is missing
func TestWebhookChannel_MissingURL(t *testing.T) {
	channel := &models.NotificationChannel{
		Name:   "test-webhook",
		Type:   "webhook",
		Config: map[string]interface{}{},
	}

	_, err := NewWebhookChannel(channel)
	if err == nil {
		t.Error("Expected error for missing URL")
	}
}

// TestWebhookChannel_Timeout tests request timeout
func TestWebhookChannel_Timeout(t *testing.T) {
	// This test would need to simulate a slow server
	// Skipping actual timeout test as it would slow down test suite

	channel := &models.NotificationChannel{
		Name: "test-webhook",
		Type: "webhook",
		Config: map[string]interface{}{
			"url": "http://localhost:9999/timeout",
		},
	}

	wc, err := NewWebhookChannel(channel)
	if err != nil {
		t.Fatalf("NewWebhookChannel failed: %v", err)
	}

	// Verify client has timeout configured
	if wc.client.Timeout != 10*time.Second {
		t.Errorf("Expected 10s timeout, got %v", wc.client.Timeout)
	}
}

// TestWebhookChannel_TypeAndName tests Type and Name methods
func TestWebhookChannel_TypeAndName(t *testing.T) {
	channel := &models.NotificationChannel{
		Name: "my-webhook",
		Type: "webhook",
		Config: map[string]interface{}{
			"url": "https://example.com",
		},
	}

	wc, err := NewWebhookChannel(channel)
	if err != nil {
		t.Fatalf("NewWebhookChannel failed: %v", err)
	}

	if wc.Type() != "webhook" {
		t.Errorf("Expected type 'webhook', got '%s'", wc.Type())
	}

	if wc.Name() != "my-webhook" {
		t.Errorf("Expected name 'my-webhook', got '%s'", wc.Name())
	}
}
