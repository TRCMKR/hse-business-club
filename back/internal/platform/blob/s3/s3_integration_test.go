//go:build integration

package s3_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/trcmkr/hse-business-club/back/internal/platform/blob/blobtest"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob/s3"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestStoreOnMinIO(t *testing.T) {
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reg := prometheus.NewRegistry()
	cfg := config.Blob{
		Endpoint:  endpoint,
		AccessKey: envOr("TEST_S3_ACCESS_KEY", "minioadmin"),
		SecretKey: envOr("TEST_S3_SECRET_KEY", "minioadmin"),
		Bucket:    fmt.Sprintf("hbc-test-%d", time.Now().UnixNano()),
		Region:    "us-east-1",
	}
	store, err := s3.New(ctx, cfg, s3.WithRegisterer(reg))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Bucket creation is idempotent.
	if _, err := s3.New(ctx, cfg); err != nil {
		t.Fatalf("New on existing bucket: %v", err)
	}

	blobtest.Run(t, store, "it/")

	n, err := testutil.GatherAndCount(reg, "blob_operations_total")
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("blob_operations_total must have series")
	}
}
