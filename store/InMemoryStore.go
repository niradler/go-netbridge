package store

import (
	"sync"
)

type InMemoryStore struct {
	data map[string]string
	mu   sync.RWMutex
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]string),
	}
}

// Init initializes the in-memory store.
func (s *InMemoryStore) Init() error {
	s.data = make(map[string]string)
	return nil
}

// Get retrieves a value from the in-memory store.
func (s *InMemoryStore) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.data[key]
	if !exists {
		return "", KeyNotFoundError
	}
	return value, nil
}

// Set sets a value in the in-memory store.
func (s *InMemoryStore) Set(key string, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

// Delete deletes a value from the in-memory store.
func (s *InMemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}
