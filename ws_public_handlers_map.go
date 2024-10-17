package bybit

import (
	"maps"
	"sync/atomic"
)

// PublicWsHandlersMap is a thread-safe map for storing WebSocket handlers using
// atomic operations on a map pointer.
type PublicWsHandlersMap[K comparable, V any] struct {
	mapPointer *atomic.Pointer[map[K]func(V) error]
}

// NewPublicWsHandlersMap initializes and returns a new instance of
// PublicWsHandlersMap with an empty handler map.
func NewPublicWsHandlersMap[K comparable, V any]() *PublicWsHandlersMap[K, V] {
	m := make(map[K]func(V) error)
	p := atomic.Pointer[map[K]func(V) error]{}
	p.Store(&m)

	return &PublicWsHandlersMap[K, V]{
		mapPointer: &p,
	}
}

// Add inserts a handler function for the given key if it does not already exist
// in the PublicWsHandlersMap.
func (m *PublicWsHandlersMap[K, V]) Add(key K, handler func(V) error) {
	for {
		handlersPointer := m.mapPointer.Load()
		handlers := *handlersPointer

		_, ok := handlers[key]
		if ok {
			return
		}

		handlersClone := maps.Clone(handlers)
		handlersClone[key] = handler

		if m.mapPointer.CompareAndSwap(handlersPointer, &handlersClone) {
			return
		}
	}
}

// Get retrieves a handler function for the given key from the
// PublicWsHandlersMap. Returns the function and true if found, otherwise nil and
// false.
func (m *PublicWsHandlersMap[K, V]) Get(key K) (func(V) error, bool) {
	handlersPointer := m.mapPointer.Load()
	handlers := *handlersPointer

	handler, ok := handlers[key]
	if !ok {
		return nil, false
	}

	return handler, true
}

// Remove deletes the handler function associated with the given key from the PublicWsHandlersMap.
func (m *PublicWsHandlersMap[K, V]) Remove(key K) {
	for {
		handlersPointer := m.mapPointer.Load()
		handlers := *handlersPointer

		_, ok := handlers[key]
		if !ok {
			return
		}

		newHandlers := make(map[K]func(V) error, len(handlers))
		for k, f := range handlers {
			if k != key {
				newHandlers[k] = f
			}
		}

		if m.mapPointer.CompareAndSwap(handlersPointer, &newHandlers) {
			return
		}
	}
}
