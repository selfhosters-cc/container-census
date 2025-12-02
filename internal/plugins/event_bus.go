package plugins

import (
	"context"
	"sync"
	"time"
)

// EventBusImpl implements the EventBus interface
type EventBusImpl struct {
	mu          sync.RWMutex
	subscribers map[string][]eventSubscriber
	nextID      int
}

type eventSubscriber struct {
	id      int
	handler EventHandler
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBusImpl {
	return &EventBusImpl{
		subscribers: make(map[string][]eventSubscriber),
	}
}

// Subscribe registers a handler for an event type
func (e *EventBusImpl) Subscribe(eventType string, handler EventHandler) func() {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := e.nextID
	e.nextID++

	e.subscribers[eventType] = append(e.subscribers[eventType], eventSubscriber{
		id:      id,
		handler: handler,
	})

	// Return unsubscribe function
	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()

		subs := e.subscribers[eventType]
		for i, sub := range subs {
			if sub.id == id {
				e.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}

// Publish sends an event to all subscribers
func (e *EventBusImpl) Publish(event Event) {
	e.mu.RLock()
	// Get subscribers for this event type
	subs := make([]eventSubscriber, len(e.subscribers[event.Type]))
	copy(subs, e.subscribers[event.Type])

	// Also notify wildcard subscribers
	wildcardSubs := make([]eventSubscriber, len(e.subscribers["*"]))
	copy(wildcardSubs, e.subscribers["*"])
	e.mu.RUnlock()

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	ctx := context.Background()

	// Notify specific subscribers
	for _, sub := range subs {
		go sub.handler(ctx, event)
	}

	// Notify wildcard subscribers
	for _, sub := range wildcardSubs {
		go sub.handler(ctx, event)
	}
}

// PublishSync sends an event and waits for all handlers to complete
func (e *EventBusImpl) PublishSync(ctx context.Context, event Event) {
	e.mu.RLock()
	subs := make([]eventSubscriber, len(e.subscribers[event.Type]))
	copy(subs, e.subscribers[event.Type])
	wildcardSubs := make([]eventSubscriber, len(e.subscribers["*"]))
	copy(wildcardSubs, e.subscribers["*"])
	e.mu.RUnlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	var wg sync.WaitGroup

	for _, sub := range subs {
		wg.Add(1)
		go func(s eventSubscriber) {
			defer wg.Done()
			s.handler(ctx, event)
		}(sub)
	}

	for _, sub := range wildcardSubs {
		wg.Add(1)
		go func(s eventSubscriber) {
			defer wg.Done()
			s.handler(ctx, event)
		}(sub)
	}

	wg.Wait()
}

// SubscriberCount returns the number of subscribers for an event type
func (e *EventBusImpl) SubscriberCount(eventType string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.subscribers[eventType])
}

// Helper functions to create common events

// NewScanCompleteEvent creates a scan complete event
func NewScanCompleteEvent(hostID int64, hostName string, containerCount int) Event {
	return Event{
		Type:      EventScanComplete,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"host_id":         hostID,
			"host_name":       hostName,
			"container_count": containerCount,
		},
	}
}

// NewContainerStateChangeEvent creates a container state change event
func NewContainerStateChangeEvent(hostID int64, containerID, containerName, oldState, newState string) Event {
	return Event{
		Type:      EventContainerStateChange,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"host_id":        hostID,
			"container_id":   containerID,
			"container_name": containerName,
			"old_state":      oldState,
			"new_state":      newState,
		},
	}
}

// NewContainerCreatedEvent creates a container created event
func NewContainerCreatedEvent(hostID int64, containerID, containerName, image string) Event {
	return Event{
		Type:      EventContainerCreated,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"host_id":        hostID,
			"container_id":   containerID,
			"container_name": containerName,
			"image":          image,
		},
	}
}

// NewContainerRemovedEvent creates a container removed event
func NewContainerRemovedEvent(hostID int64, containerID, containerName string) Event {
	return Event{
		Type:      EventContainerRemoved,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"host_id":        hostID,
			"container_id":   containerID,
			"container_name": containerName,
		},
	}
}

// NewImageUpdatedEvent creates an image updated event
func NewImageUpdatedEvent(hostID int64, containerID, containerName, oldImageID, newImageID string) Event {
	return Event{
		Type:      EventImageUpdated,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"host_id":        hostID,
			"container_id":   containerID,
			"container_name": containerName,
			"old_image_id":   oldImageID,
			"new_image_id":   newImageID,
		},
	}
}

// NewHostAddedEvent creates a host added event
func NewHostAddedEvent(hostID int64, hostName, address string) Event {
	return Event{
		Type:      EventHostAdded,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"host_id":   hostID,
			"host_name": hostName,
			"address":   address,
		},
	}
}

// NewHostRemovedEvent creates a host removed event
func NewHostRemovedEvent(hostID int64, hostName string) Event {
	return Event{
		Type:      EventHostRemoved,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"host_id":   hostID,
			"host_name": hostName,
		},
	}
}
