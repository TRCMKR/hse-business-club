// Package memory is an in-memory blob.Store for unit and e2e tests.
package memory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/trcmkr/hse-business-club/back/internal/platform/blob"
)

type object struct {
	contentType string
	data        []byte
}

// Store is a concurrency-safe in-memory blob.Store.
type Store struct {
	mu      sync.RWMutex
	objects map[string]object
}

var _ blob.Store = (*Store)(nil)

// New creates an empty store.
func New() *Store {
	return &Store{objects: make(map[string]object)}
}

// Put implements blob.Store. size is informational; the whole reader is consumed.
func (s *Store) Put(ctx context.Context, key, contentType string, r io.Reader, _ int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("blob/memory: read %s: %w", key, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = object{contentType: contentType, data: data}
	return nil
}

// Get implements blob.Store.
func (s *Store) Get(ctx context.Context, key string) (*blob.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	o, ok := s.objects[key]
	s.mu.RUnlock()
	if !ok {
		return nil, blob.ErrNotFound
	}
	return &blob.Object{
		ContentType: o.contentType,
		Size:        int64(len(o.data)),
		Body:        io.NopCloser(bytes.NewReader(o.data)),
	}, nil
}

// Delete implements blob.Store.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

// Ping implements blob.Store.
func (s *Store) Ping(ctx context.Context) error {
	return ctx.Err()
}

// Len returns the number of stored objects (for test assertions).
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.objects)
}

// Has reports whether key is stored (for test assertions).
func (s *Store) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.objects[key]
	return ok
}
