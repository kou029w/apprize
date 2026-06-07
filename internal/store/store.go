// Package store persists named Apprise configurations.
package store

import "context"

// Entry is a stored configuration identified by Key.
type Entry struct {
	Key       string
	Format    string // "text" | "yaml"
	Config    string // raw configuration body
	UpdatedAt int64  // unix seconds
}

// Store is the persistence interface used by the persistent endpoints.
//
// Implementations must be safe for concurrent use.
type Store interface {
	// Get returns the entry for key. The bool is false when no entry exists.
	Get(ctx context.Context, key string) (Entry, bool, error)
	// Put inserts or replaces the entry.
	Put(ctx context.Context, e Entry) error
	// Delete removes key, returning whether it existed.
	Delete(ctx context.Context, key string) (bool, error)
	// List returns all stored keys.
	List(ctx context.Context) ([]string, error)
	// Close releases any underlying resources.
	Close() error
}
