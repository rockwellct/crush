package ext

import (
	"fmt"
	"sync"
)

// EventBus manages subscription and dispatch of lifecycle events across extensions.
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

// NewEventBus creates an initialized EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
	}
}

// Subscribe registers an event handler for a specific event name.
func (b *EventBus) Subscribe(event string, handler EventHandler) {
	if handler == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[event] = append(b.handlers[event], handler)
}

// Emit dispatches an event to all registered handlers for the event.
// Handlers are executed in order. If a handler returns a ToolDecision (e.g. Deny),
// or a modified value (e.g. for EventInput), that result is returned.
func (b *EventBus) Emit(evt Event) (any, error) {
	b.mu.RLock()
	handlers := append([]EventHandler(nil), b.handlers[evt.Name]...)
	b.mu.RUnlock()

	var lastResult any
	for _, handler := range handlers {
		res, err := handler(evt)
		if err != nil {
			return nil, fmt.Errorf("event handler error on %s: %w", evt.Name, err)
		}

		if res != nil {
			lastResult = res
			// If a tool decision is Deny, stop early and return Deny
			if dec, ok := res.(ToolDecision); ok && dec == Deny {
				return dec, nil
			}
			if strDec, ok := res.(string); ok && ToolDecision(strDec) == Deny {
				return Deny, nil
			}
			// If input event modified prompt, update evt.Prompt for subsequent handlers
			if evt.Name == EventInput {
				if s, ok := res.(string); ok {
					evt.Prompt = s
				}
			}
		}
	}

	return lastResult, nil
}
