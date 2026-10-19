// Package blobtest is a conformance suite for blob.Store implementations.
package blobtest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/trcmkr/hse-business-club/back/internal/platform/blob"
)

// Run checks the blob.Store contract. Keys are prefixed with prefix to avoid clashes.
func Run(t *testing.T, s blob.Store, prefix string) {
	t.Helper()
	ctx := context.Background()
	key := prefix + "photo.jpg"
	data := []byte("\xff\xd8\xff fake jpeg")

	if err := s.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	if _, err := s.Get(ctx, key); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("Get missing: err = %v, want ErrNotFound", err)
	}

	if err := s.Put(ctx, key, "image/jpeg", bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("Put: %v", err)
	}

	obj, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, err := io.ReadAll(obj.Body)
	_ = obj.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("body = %q, want %q", got, data)
	}
	if obj.ContentType != "image/jpeg" {
		t.Fatalf("ContentType = %q", obj.ContentType)
	}
	if obj.Size != int64(len(data)) {
		t.Fatalf("Size = %d, want %d", obj.Size, len(data))
	}

	// Overwrite.
	data2 := []byte("second")
	if err := s.Put(ctx, key, "image/jpeg", bytes.NewReader(data2), int64(len(data2))); err != nil {
		t.Fatalf("Put overwrite: %v", err)
	}
	obj, err = s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	got, _ = io.ReadAll(obj.Body)
	_ = obj.Body.Close()
	if !bytes.Equal(got, data2) {
		t.Fatalf("body after overwrite = %q", got)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, key); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("Get after delete: err = %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete missing must not fail: %v", err)
	}
}
