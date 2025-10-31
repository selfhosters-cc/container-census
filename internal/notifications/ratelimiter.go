package notifications

import (
	"context"
	"log"
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting with batching support
type RateLimiter struct {
	maxPerHour    int
	batchInterval time.Duration
	tokens        int
	lastReset     time.Time
	batchQueue    []notificationTask
	mu            sync.Mutex
	batchTicker   *time.Ticker
	stopBatch     chan struct{}
	notifier      *NotificationService
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxPerHour int, batchInterval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		maxPerHour:    maxPerHour,
		batchInterval: batchInterval,
		tokens:        maxPerHour,
		lastReset:     time.Now(),
		batchQueue:    make([]notificationTask, 0),
		stopBatch:     make(chan struct{}),
	}

	// Start batch processor
	go rl.runBatchProcessor()

	return rl
}

// Allow checks if a notification can be sent immediately
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens if hour has passed
	if time.Since(rl.lastReset) >= time.Hour {
		rl.tokens = rl.maxPerHour
		rl.lastReset = time.Now()
	}

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// AddToBatch adds a notification to the batch queue
func (rl *RateLimiter) AddToBatch(task notificationTask) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.batchQueue = append(rl.batchQueue, task)
}

// runBatchProcessor sends batched notifications every interval
func (rl *RateLimiter) runBatchProcessor() {
	ticker := time.NewTicker(rl.batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.sendBatch()
		case <-rl.stopBatch:
			return
		}
	}
}

// sendBatch sends a summary notification for all queued notifications
func (rl *RateLimiter) sendBatch() {
	rl.mu.Lock()
	if len(rl.batchQueue) == 0 {
		rl.mu.Unlock()
		return
	}

	// Copy and clear queue
	batch := make([]notificationTask, len(rl.batchQueue))
	copy(batch, rl.batchQueue)
	rl.batchQueue = rl.batchQueue[:0]
	rl.mu.Unlock()

	log.Printf("Rate limiter: Sending batch notification for %d queued events", len(batch))

	// Group by channel
	channelGroups := make(map[int64][]notificationTask)
	for _, task := range batch {
		channelGroups[task.Channel] = append(channelGroups[task.Channel], task)
	}

	// Send one summary per channel
	for channelID, tasks := range channelGroups {
		if rl.notifier != nil {
			ctx := context.Background()
			go rl.sendBatchSummary(ctx, channelID, tasks)
		}
	}
}

// sendBatchSummary sends a summary notification for a batch
func (rl *RateLimiter) sendBatchSummary(ctx context.Context, channelID int64, tasks []notificationTask) {
	if rl.notifier == nil {
		return
	}

	// Count event types
	eventCounts := make(map[string]int)
	for _, task := range tasks {
		eventCounts[task.Event.EventType]++
	}

	// Build summary message
	message := "📊 Notification Summary:\n"
	for eventType, count := range eventCounts {
		message += "  • " + eventType + ": " + string(rune(count)) + "\n"
	}

	// Send as a single notification
	// (Implementation would use the first task's rule for context)
	if len(tasks) > 0 {
		firstTask := tasks[0]
		rl.notifier.sendSingleNotification(ctx, firstTask)
	}
}

// Stop stops the batch processor
func (rl *RateLimiter) Stop() {
	close(rl.stopBatch)
}

// GetRemaining returns remaining tokens
func (rl *RateLimiter) GetRemaining() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill if needed
	if time.Since(rl.lastReset) >= time.Hour {
		rl.tokens = rl.maxPerHour
		rl.lastReset = time.Now()
	}

	return rl.tokens
}

// GetResetTime returns when tokens will be refilled
func (rl *RateLimiter) GetResetTime() time.Time {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return rl.lastReset.Add(time.Hour)
}

// SetNotifier sets the notification service reference for batch sending
func (rl *RateLimiter) SetNotifier(ns *NotificationService) {
	rl.notifier = ns
}
