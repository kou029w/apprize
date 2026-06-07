package store

import (
	"context"
	"sort"
	"sync"
)

// Memory is an in-memory Store used for tests and when DBPath is :memory:.
type Memory struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{entries: make(map[string]Entry)}
}

func (m *Memory) Get(_ context.Context, key string) (Entry, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[key]
	return e, ok, nil
}

func (m *Memory) Put(_ context.Context, e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[e.Key] = e
	return nil
}

func (m *Memory) Delete(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.entries[key]
	delete(m.entries, key)
	return ok, nil
}

func (m *Memory) List(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.entries))
	for k := range m.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *Memory) Close() error { return nil }
