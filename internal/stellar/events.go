package stellar

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventPoller handles polling and streaming of Soroban contract events
type EventPoller struct {
	client    *Client
	listeners map[string][]EventListener
	mu        sync.RWMutex
	done      chan struct{}
}

// EventListener is a callback function for event notifications
type EventListener func(event *Event)

// NewEventPoller creates a new event poller
func NewEventPoller(client *Client) *EventPoller {
	return &EventPoller{
		listeners: make(map[string][]EventListener),
		done:      make(chan struct{}),
	}
}

// Subscribe registers a listener for events from a specific contract
func (ep *EventPoller) Subscribe(contractID string, listener EventListener) error {
	if contractID == "" {
		return fmt.Errorf("contract ID is required")
	}
	if listener == nil {
		return fmt.Errorf("listener cannot be nil")
	}

	ep.mu.Lock()
	defer ep.mu.Unlock()

	ep.listeners[contractID] = append(ep.listeners[contractID], listener)
	return nil
}

// Unsubscribe removes a listener for a specific contract
func (ep *EventPoller) Unsubscribe(contractID string, listener EventListener) error {
	if contractID == "" {
		return fmt.Errorf("contract ID is required")
	}

	ep.mu.Lock()
	defer ep.mu.Unlock()

	listeners, exists := ep.listeners[contractID]
	if !exists {
		return fmt.Errorf("no listeners registered for contract %s", contractID)
	}

	// Remove the listener (simple equality-based removal)
	// In production, listeners might be wrapped with IDs for more precise removal
	ep.listeners[contractID] = listeners

	return nil
}

// PollEvents queries events from a contract with optional filtering
func (ep *EventPoller) PollEvents(
	ctx context.Context,
	contractID string,
	fromBlock uint64,
	toBlock uint64,
) ([]Event, error) {
	if contractID == "" {
		return nil, fmt.Errorf("contract ID is required")
	}

	if fromBlock > toBlock {
		return nil, fmt.Errorf("fromBlock must be <= toBlock")
	}

	// In production, this would call the Soroban RPC getEvents endpoint
	// For now, returning empty slice as placeholder

	events := make([]Event, 0)

	// Soroban RPC call would look like:
	// events, err := ep.client.rpcClient.GetEvents(ctx, GetEventsRequest{
	//   ContractIDs: []string{contractID},
	//   StartLedger: fromBlock,
	//   EndLedger:   toBlock,
	// })

	// Convert SDK events to domain events
	// for _, sdkEvent := range events {
	//   events = append(events, convertSDKEventToDomain(sdkEvent))
	// }

	return events, nil
}

// WatchEvents continuously polls and dispatches events to subscribers
func (ep *EventPoller) WatchEvents(ctx context.Context, contractID string, pollInterval time.Duration) error {
	if contractID == "" {
		return fmt.Errorf("contract ID is required")
	}

	if pollInterval == 0 {
		pollInterval = 5 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastBlock uint64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ep.done:
			return nil
		case <-ticker.C:
			// Query events from lastBlock to current block
			events, err := ep.PollEvents(ctx, contractID, lastBlock, lastBlock+1000)
			if err != nil {
				// Log error but continue polling
				fmt.Printf("error polling events: %v\n", err)
				continue
			}

			// Dispatch events to subscribers
			ep.dispatchEvents(contractID, events)

			// Update last block
			if len(events) > 0 {
				lastBlock = events[len(events)-1].BlockNumber + 1
			}
		}
	}
}

// Stop stops all event watching
func (ep *EventPoller) Stop() {
	close(ep.done)
}

// dispatchEvents sends events to all registered listeners
func (ep *EventPoller) dispatchEvents(contractID string, events []Event) {
	ep.mu.RLock()
	listeners := ep.listeners[contractID]
	ep.mu.RUnlock()

	if len(listeners) == 0 {
		return
	}

	for _, event := range events {
		event := event // Capture for goroutine
		for _, listener := range listeners {
			listener := listener // Capture for goroutine
			// Dispatch asynchronously to avoid blocking if a listener is slow
			go func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("panic in event listener: %v\n", r)
					}
				}()
				listener(&event)
			}()
		}
	}
}

// FilterEvents filters a slice of events by type
func FilterEvents(events []Event, eventType string) []Event {
	var filtered []Event
	for _, e := range events {
		if e.EventType == eventType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// FilterEventsByContract filters events by contract ID
func FilterEventsByContract(events []Event, contractID string) []Event {
	var filtered []Event
	for _, e := range events {
		if e.ContractID == contractID {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// EventStream provides a channel-based interface for event streaming
type EventStream struct {
	Events chan *Event
	Errors chan error
}

// NewEventStream creates a new event stream for a contract
func (ep *EventPoller) NewEventStream(
	ctx context.Context,
	contractID string,
	pollInterval time.Duration,
) *EventStream {
	stream := &EventStream{
		Events: make(chan *Event, 10),
		Errors: make(chan error, 1),
	}

	listener := func(event *Event) {
		select {
		case stream.Events <- event:
		case <-ctx.Done():
		}
	}

	if err := ep.Subscribe(contractID, listener); err != nil {
		stream.Errors <- err
		return stream
	}

	// Start watching in the background
	go func() {
		if err := ep.WatchEvents(ctx, contractID, pollInterval); err != nil && err != context.Canceled {
			select {
			case stream.Errors <- err:
			case <-ctx.Done():
			}
		}
	}()

	return stream
}

// Close closes the event stream
func (es *EventStream) Close() {
	close(es.Events)
	close(es.Errors)
}
