// Package blob is the platform port for file storage (profile photos).
// Implementations: blob/s3 (S3-compatible, MinIO in compose) and blob/memory (tests).
package blob

import (
	"context"
	"errors"
	"io"
)

// Object is a stored file. The caller must close Body.
type Object struct {
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

// Store stores files by key.
type Store interface {
	Put(ctx context.Context, key, contentType string, r io.Reader, size int64) error
	Get(ctx context.Context, key string) (*Object, error) // ErrNotFound
	Delete(ctx context.Context, key string) error         // отсутствие — не ошибка
	Ping(ctx context.Context) error                       // для /readyz
}

// ErrNotFound is returned by Store.Get when there is no object with the key.
var ErrNotFound = errors.New("blob: not found")
