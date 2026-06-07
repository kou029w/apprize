package store_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"git.fogtype.com/nebel/apprize/internal/store"
)

// newStores returns one instance of every Store implementation so the shared
// suite runs identically against memory and sqlite (file and :memory:).
func newStores(t *testing.T) map[string]store.Store {
	t.Helper()
	file, err := store.OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite file: %v", err)
	}
	mem, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite memory: %v", err)
	}
	stores := map[string]store.Store{
		"memory":        store.NewMemory(),
		"sqlite-file":   file,
		"sqlite-memory": mem,
	}
	t.Cleanup(func() {
		for _, s := range stores {
			_ = s.Close()
		}
	})
	return stores
}

func TestStoreCRUD(t *testing.T) {
	ctx := context.Background()
	for name, s := range newStores(t) {
		t.Run(name, func(t *testing.T) {
			if _, ok, err := s.Get(ctx, "k"); err != nil || ok {
				t.Fatalf("empty Get: ok=%v err=%v", ok, err)
			}

			seed := store.Entry{Key: "k", Format: "text", Config: "gotify://h/t", UpdatedAt: 1}
			if err := s.Put(ctx, seed); err != nil {
				t.Fatalf("Put: %v", err)
			}
			got, ok, err := s.Get(ctx, "k")
			if err != nil || !ok || got != seed {
				t.Fatalf("Get after Put = %+v ok=%v err=%v", got, ok, err)
			}

			// Put upserts in place rather than duplicating the key.
			if err := s.Put(ctx, store.Entry{Key: "k", Format: "yaml", Config: "urls: []", UpdatedAt: 2}); err != nil {
				t.Fatalf("upsert: %v", err)
			}
			keys, err := s.List(ctx)
			if err != nil || len(keys) != 1 || keys[0] != "k" {
				t.Fatalf("List after upsert = %v err=%v", keys, err)
			}

			deleted, err := s.Delete(ctx, "k")
			if err != nil || !deleted {
				t.Fatalf("Delete existing: deleted=%v err=%v", deleted, err)
			}
			if again, err := s.Delete(ctx, "k"); err != nil || again {
				t.Fatalf("Delete missing: deleted=%v err=%v", again, err)
			}
		})
	}
}

// TestSQLiteMemoryConcurrent guards the shared-cache fix: a bare ":memory:" DSN
// would give each pooled connection its own database, so concurrent reads would
// hit "no such table".
func TestSQLiteMemoryConcurrent(t *testing.T) {
	ctx := context.Background()
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	if err := s.Put(ctx, store.Entry{Key: "k", Format: "text", Config: "gotify://h/t", UpdatedAt: 1}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	var wg sync.WaitGroup
	fails := make(chan error, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok, err := s.Get(ctx, "k"); err != nil || !ok {
				fails <- fmt.Errorf("ok=%v err=%v", ok, err)
			}
		}()
	}
	wg.Wait()
	close(fails)
	if err := <-fails; err != nil {
		t.Fatalf("concurrent Get: %v", err)
	}
}
