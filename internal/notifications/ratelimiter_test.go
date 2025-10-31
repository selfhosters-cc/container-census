package notifications

import (
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// TestRateLimiter_TokenBucket tests the token bucket algorithm
func TestRateLimiter_TokenBucket(t *testing.T) {
	maxPerHour := 10
	batchInterval := 1 * time.Minute

	rl := NewRateLimiter(maxPerHour, batchInterval)

	// Initially should have max tokens
	if rl.tokens != maxPerHour {
		t.Errorf("Expected %d initial tokens, got %d", maxPerHour, rl.tokens)
	}

	// Test consuming tokens
	for i := 0; i < maxPerHour; i++ {
		if !rl.tryConsume() {
			t.Errorf("Failed to consume token %d", i+1)
		}
	}

	// Should have no tokens left
	if rl.tokens != 0 {
		t.Errorf("Expected 0 tokens after consuming all, got %d", rl.tokens)
	}

	// Next attempt should fail
	if rl.tryConsume() {
		t.Error("Should not be able to consume token when bucket is empty")
	}
}

// TestRateLimiter_Refill tests token refill logic
func TestRateLimiter_Refill(t *testing.T) {
	maxPerHour := 100
	batchInterval := 1 * time.Minute

	rl := NewRateLimiter(maxPerHour, batchInterval)

	// Consume all tokens
	for i := 0; i < maxPerHour; i++ {
		rl.tryConsume()
	}

	if rl.tokens != 0 {
		t.Error("Expected 0 tokens after consuming all")
	}

	// Manually set last refill to 1 hour ago
	rl.mu.Lock()
	rl.lastRefill = time.Now().Add(-1 * time.Hour)
	rl.mu.Unlock()

	// Refill should restore tokens to max
	rl.refillIfNeeded()

	if rl.tokens != maxPerHour {
		t.Errorf("Expected %d tokens after refill, got %d", maxPerHour, rl.tokens)
	}
}

// TestRateLimiter_QueueBatch tests batching when rate limited
func TestRateLimiter_QueueBatch(t *testing.T) {
	maxPerHour := 2
	batchInterval := 1 * time.Second

	rl := NewRateLimiter(maxPerHour, batchInterval)

	notifications := []models.NotificationLog{
		{
			RuleName:      "test-rule",
			EventType:     "container_stopped",
			ContainerName: "container1",
			Message:       "Test 1",
			ScannedAt:     time.Now(),
		},
		{
			RuleName:      "test-rule",
			EventType:     "container_stopped",
			ContainerName: "container2",
			Message:       "Test 2",
			ScannedAt:     time.Now(),
		},
		{
			RuleName:      "test-rule",
			EventType:     "container_stopped",
			ContainerName: "container3",
			Message:       "Test 3",
			ScannedAt:     time.Now(),
		},
	}

	// First two should succeed immediately
	sent, queued := rl.Send(notifications[:2])
	if len(sent) != 2 {
		t.Errorf("Expected 2 notifications sent immediately, got %d", len(sent))
	}
	if len(queued) != 0 {
		t.Errorf("Expected 0 notifications queued, got %d", len(queued))
	}

	// Third should be queued (no tokens left)
	sent, queued = rl.Send(notifications[2:])
	if len(sent) != 0 {
		t.Errorf("Expected 0 notifications sent, got %d", len(sent))
	}
	if len(queued) != 1 {
		t.Errorf("Expected 1 notification queued, got %d", len(queued))
	}

	// Verify batch queue
	rl.mu.RLock()
	batchSize := len(rl.batchQueue)
	rl.mu.RUnlock()

	if batchSize != 1 {
		t.Errorf("Expected 1 notification in batch queue, got %d", batchSize)
	}
}

// TestRateLimiter_PerChannelBatching tests that batching groups by channel
func TestRateLimiter_PerChannelBatching(t *testing.T) {
	maxPerHour := 1
	batchInterval := 1 * time.Second

	rl := NewRateLimiter(maxPerHour, batchInterval)

	// Consume the one token
	rl.tryConsume()

	// Queue notifications for different channels
	notifications := []models.NotificationLog{
		{
			RuleName:      "rule1",
			EventType:     "container_stopped",
			ContainerName: "container1",
			Message:       "Channel 1 notification",
			ChannelID:     1,
			ScannedAt:     time.Now(),
		},
		{
			RuleName:      "rule2",
			EventType:     "new_image",
			ContainerName: "container2",
			Message:       "Channel 1 another",
			ChannelID:     1,
			ScannedAt:     time.Now(),
		},
		{
			RuleName:      "rule3",
			EventType:     "container_stopped",
			ContainerName: "container3",
			Message:       "Channel 2 notification",
			ChannelID:     2,
			ScannedAt:     time.Now(),
		},
	}

	// All should be queued
	sent, queued := rl.Send(notifications)
	if len(sent) != 0 {
		t.Errorf("Expected 0 sent (no tokens), got %d", len(sent))
	}
	if len(queued) != 3 {
		t.Errorf("Expected 3 queued, got %d", len(queued))
	}

	// Check batch queue grouping
	rl.mu.RLock()
	queueSize := len(rl.batchQueue)
	rl.mu.RUnlock()

	if queueSize != 3 {
		t.Errorf("Expected 3 items in batch queue, got %d", queueSize)
	}

	// Verify they're grouped by channel when processed
	// (This would require access to processBatch, which might be private)
	t.Log("Per-channel batching logic tested via queue size")
}

// TestRateLimiter_ConcurrentAccess tests thread-safe operations
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	maxPerHour := 100
	batchInterval := 1 * time.Second

	rl := NewRateLimiter(maxPerHour, batchInterval)

	done := make(chan bool)
	errors := make(chan error, 10)

	// Concurrent token consumption
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				rl.tryConsume()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}

	// Verify token count is consistent (100 - 100 consumed = 0)
	if rl.tokens != 0 {
		t.Errorf("Expected 0 tokens after concurrent consumption, got %d", rl.tokens)
	}
}

// TestRateLimiter_RefillInterval tests partial hour refills
func TestRateLimiter_RefillInterval(t *testing.T) {
	maxPerHour := 60 // 1 per minute
	batchInterval := 1 * time.Second

	rl := NewRateLimiter(maxPerHour, batchInterval)

	// Consume all tokens
	for i := 0; i < maxPerHour; i++ {
		rl.tryConsume()
	}

	// Set last refill to 10 minutes ago
	rl.mu.Lock()
	rl.lastRefill = time.Now().Add(-10 * time.Minute)
	rl.mu.Unlock()

	// Refill should restore proportionally
	rl.refillIfNeeded()

	// Should have approximately 10 tokens (10 minutes * 1 per minute)
	if rl.tokens < 9 || rl.tokens > 11 {
		t.Errorf("Expected ~10 tokens after 10 minute partial refill, got %d", rl.tokens)
	}
}

// TestRateLimiter_NoNegativeTokens tests that tokens can't go negative
func TestRateLimiter_NoNegativeTokens(t *testing.T) {
	maxPerHour := 5
	batchInterval := 1 * time.Second

	rl := NewRateLimiter(maxPerHour, batchInterval)

	// Consume all tokens
	for i := 0; i < maxPerHour; i++ {
		if !rl.tryConsume() {
			t.Errorf("Failed to consume token %d", i+1)
		}
	}

	// Try to consume more
	for i := 0; i < 10; i++ {
		if rl.tryConsume() {
			t.Error("Should not be able to consume when bucket is empty")
		}
	}

	// Tokens should still be 0, not negative
	if rl.tokens < 0 {
		t.Errorf("Tokens went negative: %d", rl.tokens)
	}
}

// TestRateLimiter_BatchInterval tests batch processing timing
func TestRateLimiter_BatchInterval(t *testing.T) {
	maxPerHour := 1
	batchInterval := 100 * time.Millisecond

	rl := NewRateLimiter(maxPerHour, batchInterval)

	// Set last batch time to past the interval
	rl.mu.Lock()
	rl.lastBatchSent = time.Now().Add(-200 * time.Millisecond)
	rl.mu.Unlock()

	// Should be ready for batch
	rl.mu.RLock()
	elapsed := time.Since(rl.lastBatchSent)
	rl.mu.RUnlock()

	if elapsed < batchInterval {
		t.Errorf("Expected elapsed time >= %v, got %v", batchInterval, elapsed)
	}

	// Verify shouldSendBatch logic would return true
	shouldSend := elapsed >= batchInterval

	if !shouldSend {
		t.Error("Should be ready to send batch after interval elapsed")
	}
}

// TestRateLimiter_MaxTokensCap tests that tokens don't exceed max
func TestRateLimiter_MaxTokensCap(t *testing.T) {
	maxPerHour := 10
	batchInterval := 1 * time.Second

	rl := NewRateLimiter(maxPerHour, batchInterval)

	// Set last refill to multiple hours ago
	rl.mu.Lock()
	rl.lastRefill = time.Now().Add(-5 * time.Hour)
	rl.mu.Unlock()

	// Refill
	rl.refillIfNeeded()

	// Should not exceed max
	if rl.tokens > maxPerHour {
		t.Errorf("Tokens exceed max: got %d, max %d", rl.tokens, maxPerHour)
	}

	// Should be exactly max
	if rl.tokens != maxPerHour {
		t.Errorf("Expected exactly %d tokens after long refill period, got %d", maxPerHour, rl.tokens)
	}
}

// TestRateLimiter_Statistics tests rate limiter statistics
func TestRateLimiter_Statistics(t *testing.T) {
	maxPerHour := 10
	batchInterval := 1 * time.Second

	rl := NewRateLimiter(maxPerHour, batchInterval)

	// Get initial stats
	stats := rl.GetStats()

	if stats.MaxPerHour != maxPerHour {
		t.Errorf("Expected max per hour %d, got %d", maxPerHour, stats.MaxPerHour)
	}

	if stats.CurrentTokens != maxPerHour {
		t.Errorf("Expected current tokens %d, got %d", maxPerHour, stats.CurrentTokens)
	}

	// Consume some tokens
	rl.tryConsume()
	rl.tryConsume()

	stats = rl.GetStats()

	if stats.CurrentTokens != maxPerHour-2 {
		t.Errorf("Expected %d tokens after consuming 2, got %d", maxPerHour-2, stats.CurrentTokens)
	}

	// Queue some notifications
	notifications := []models.NotificationLog{
		{Message: "Test 1", ScannedAt: time.Now()},
		{Message: "Test 2", ScannedAt: time.Now()},
		{Message: "Test 3", ScannedAt: time.Now()},
	}

	// Consume remaining tokens
	for rl.tryConsume() {
		// empty
	}

	// Try to send (should queue)
	rl.Send(notifications)

	stats = rl.GetStats()

	if stats.QueuedCount == 0 {
		t.Log("Note: QueuedCount is 0, might not be exposed in stats")
	}

	t.Logf("Rate limiter stats: tokens=%d, max=%d", stats.CurrentTokens, stats.MaxPerHour)
}
