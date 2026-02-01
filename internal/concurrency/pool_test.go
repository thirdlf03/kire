package concurrency_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/thirdlf03/kire/internal/concurrency"
)

func TestPool_ConcurrencyLimit(t *testing.T) {
	pool := concurrency.NewPool(2, 0)
	ctx := context.Background()

	var maxConcurrent int64
	var current int64

	results, err := concurrency.Map(ctx, pool, 10, func(i int) (int, error) {
		c := atomic.AddInt64(&current, 1)
		for {
			old := atomic.LoadInt64(&maxConcurrent)
			if c > old {
				if atomic.CompareAndSwapInt64(&maxConcurrent, old, c) {
					break
				}
			} else {
				break
			}
		}
		// simulate work
		for j := 0; j < 1000; j++ {
			_ = j * j
		}
		atomic.AddInt64(&current, -1)
		return i * 2, nil
	})

	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	for i, r := range results {
		if r != i*2 {
			t.Errorf("result[%d]: expected %d, got %d", i, i*2, r)
		}
	}

	if maxConcurrent > 2 {
		t.Errorf("concurrency exceeded limit: max was %d", maxConcurrent)
	}
}

func TestMap_ErrorAggregation(t *testing.T) {
	pool := concurrency.NewPool(10, 0)
	ctx := context.Background()

	_, err := concurrency.Map(ctx, pool, 5, func(i int) (int, error) {
		if i == 1 || i == 3 {
			return 0, fmt.Errorf("error-%d", i)
		}
		return i, nil
	})

	if err == nil {
		t.Fatal("expected error")
	}
	// At least one of the real errors should be in the message
	errStr := err.Error()
	if !strings.Contains(errStr, "error-1") && !strings.Contains(errStr, "error-3") {
		t.Errorf("expected at least one real error message, got: %s", errStr)
	}
}

func TestPool_ContextCancellation(t *testing.T) {
	pool := concurrency.NewPool(1, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := concurrency.Map(ctx, pool, 5, func(i int) (int, error) {
		return i, nil
	})

	if err == nil {
		t.Error("expected error from cancelled context")
	}
}
