// Package s3 is a blob.Store on an S3-compatible storage (MinIO) via minio-go.
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/trcmkr/hse-business-club/back/internal/platform/blob"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
)

// Operation label values of blob metrics.
const (
	opPut    = "put"
	opGet    = "get"
	opDelete = "delete"
	opPing   = "ping"
)

// Store is a blob.Store backed by an S3 bucket.
type Store struct {
	client  *minio.Client
	bucket  string
	metrics *metrics
}

var _ blob.Store = (*Store)(nil)

type options struct {
	registerer prometheus.Registerer
	transport  http.RoundTripper
}

// Option configures New.
type Option func(*options)

// WithRegisterer registers blob_operations_total and blob_operation_duration_seconds
// in r. Without it metrics are collected but not exposed.
func WithRegisterer(r prometheus.Registerer) Option {
	return func(o *options) { o.registerer = r }
}

// WithTransport sets the base HTTP transport (it is still wrapped with otelhttp).
func WithTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.transport = rt }
}

// New creates the client (transport wrapped with otelhttp, using the global tracer
// provider) and creates the bucket if it does not exist.
func New(ctx context.Context, cfg config.Blob, opts ...Option) (*Store, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	base := o.transport
	if base == nil {
		t, err := minio.DefaultTransport(cfg.UseSSL)
		if err != nil {
			return nil, fmt.Errorf("blob/s3: transport: %w", err)
		}
		base = t
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:    cfg.UseSSL,
		Region:    cfg.Region,
		Transport: otelhttp.NewTransport(base),
	})
	if err != nil {
		return nil, fmt.Errorf("blob/s3: client: %w", err)
	}

	m := newMetrics()
	if o.registerer != nil {
		if err := m.register(o.registerer); err != nil {
			return nil, err
		}
	}

	s := &Store{client: client, bucket: cfg.Bucket, metrics: m}
	if err := s.ensureBucket(ctx, cfg.Region); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureBucket(ctx context.Context, region string) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("blob/s3: bucket exists %s: %w", s.bucket, err)
	}
	if exists {
		return nil
	}
	err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: region})
	if err != nil {
		// Another instance may have created it concurrently.
		if code := minio.ToErrorResponse(err).Code; code == "BucketAlreadyOwnedByYou" || code == "BucketAlreadyExists" {
			return nil
		}
		return fmt.Errorf("blob/s3: make bucket %s: %w", s.bucket, err)
	}
	return nil
}

// Bucket returns the bucket name.
func (s *Store) Bucket() string {
	return s.bucket
}

// Put implements blob.Store. size may be -1 if unknown.
func (s *Store) Put(ctx context.Context, key, contentType string, r io.Reader, size int64) (err error) {
	defer s.metrics.observe(opPut, time.Now(), &err)
	_, err = s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("blob/s3: put %s: %w", key, err)
	}
	return nil
}

// Get implements blob.Store. Missing key yields blob.ErrNotFound.
func (s *Store) Get(ctx context.Context, key string) (_ *blob.Object, err error) {
	defer s.metrics.observe(opGet, time.Now(), &err)
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, s.mapErr(key, err)
	}
	// GetObject is lazy: Stat performs the request and reveals NoSuchKey.
	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, s.mapErr(key, err)
	}
	return &blob.Object{ContentType: info.ContentType, Size: info.Size, Body: obj}, nil
}

// Delete implements blob.Store. Deleting a missing key is not an error.
func (s *Store) Delete(ctx context.Context, key string) (err error) {
	defer s.metrics.observe(opDelete, time.Now(), &err)
	err = s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("blob/s3: delete %s: %w", key, err)
	}
	return nil
}

// Ping implements blob.Store: the bucket must exist and be reachable.
func (s *Store) Ping(ctx context.Context) (err error) {
	defer s.metrics.observe(opPing, time.Now(), &err)
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("blob/s3: ping: %w", err)
	}
	if !ok {
		return fmt.Errorf("blob/s3: bucket %s does not exist", s.bucket)
	}
	return nil
}

func (s *Store) mapErr(key string, err error) error {
	if isNotFound(err) {
		return blob.ErrNotFound
	}
	return fmt.Errorf("blob/s3: get %s: %w", key, err)
}

func isNotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.Code == "NoSuchKey" || (resp.StatusCode == http.StatusNotFound && resp.Code != "NoSuchBucket")
}

type metrics struct {
	ops      *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func newMetrics() *metrics {
	return &metrics{
		ops: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "blob_operations_total",
			Help: "Blob storage operations by operation and status (ok, not_found, error).",
		}, []string{"op", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "blob_operation_duration_seconds",
			Help:    "Blob storage operation duration.",
			Buckets: prometheus.DefBuckets,
		}, []string{"op"}),
	}
}

func (m *metrics) register(r prometheus.Registerer) error {
	for _, c := range []prometheus.Collector{m.ops, m.duration} {
		if err := r.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return fmt.Errorf("blob/s3: register metrics: %w", err)
			}
		}
	}
	return nil
}

func (m *metrics) observe(op string, start time.Time, errp *error) {
	st := "ok"
	if err := *errp; err != nil {
		st = "error"
		if errors.Is(err, blob.ErrNotFound) {
			st = "not_found"
		}
	}
	m.ops.WithLabelValues(op, st).Inc()
	m.duration.WithLabelValues(op).Observe(time.Since(start).Seconds())
}
