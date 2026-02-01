package concurrency

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/time/rate"
)

// Pool provides concurrency-limited, rate-limited task execution.
type Pool struct {
	sem     chan struct{}
	limiter *rate.Limiter
}

// NewPool creates a pool with the given concurrency and QPS limits.
// If qps <= 0, no rate limiting is applied.
func NewPool(concurrency int, qps float64) *Pool {
	if concurrency <= 0 {
		concurrency = 1
	}
	var limiter *rate.Limiter
	if qps > 0 {
		limiter = rate.NewLimiter(rate.Limit(qps), int(qps)+1)
	}
	return &Pool{
		sem:     make(chan struct{}, concurrency),
		limiter: limiter,
	}
}

// Do executes the function with concurrency and rate limiting.
func (p *Pool) Do(ctx context.Context, fn func() error) error {
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return err
		}
	}
	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
		return fn()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Map executes n tasks in parallel using the pool, collecting results and errors.
// On the first task error, remaining tasks are cancelled. All non-cancellation
// errors are aggregated via errors.Join.
func Map[T any](ctx context.Context, pool *Pool, n int, fn func(i int) (T, error)) ([]T, error) {
	innerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]T, n)
	errs := make([]error, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Go(func() {
			err := pool.Do(innerCtx, func() error {
				val, err := fn(i)
				if err != nil {
					return err
				}
				results[i] = val
				return nil
			})
			if err != nil {
				errs[i] = err
				cancel() // cancel remaining tasks on first error
			}
		})
	}

	wg.Wait()

	// If the parent context was cancelled, return its error directly
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Collect non-cancellation errors (from internal cancel)
	var realErrs []error
	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			realErrs = append(realErrs, err)
		}
	}
	if len(realErrs) > 0 {
		return nil, errors.Join(realErrs...)
	}

	return results, nil
}
