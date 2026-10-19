package memory_test

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/trcmkr/hse-business-club/back/internal/platform/blob/blobtest"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob/memory"
)

func TestStoreContract(t *testing.T) {
	t.Parallel()
	blobtest.Run(t, memory.New(), "")
}

func TestStoreConcurrent(t *testing.T) {
	t.Parallel()

	s := memory.New()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Go(func() {
			key := fmt.Sprintf("k%d", i%5)
			_ = s.Put(ctx, key, "text/plain", bytes.NewReader([]byte(key)), 2)
			if obj, err := s.Get(ctx, key); err == nil {
				_ = obj.Body.Close()
			}
			_ = s.Delete(ctx, key)
		})
	}
	wg.Wait()
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0", s.Len())
	}
}
