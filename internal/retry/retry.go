package retry

import (
	"context"
	"time"

	"github.com/jasonz/webscout/internal/backend"
)

type Policy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	Multiplier   float64
	MaxDelay     time.Duration
}

func (p Policy) normalized() Policy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.InitialDelay <= 0 {
		p.InitialDelay = 500 * time.Millisecond
	}
	if p.Multiplier <= 1 {
		p.Multiplier = 2
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 5 * time.Second
	}
	return p
}

func Do(ctx context.Context, policy Policy, fn func() error) error {
	_, err := DoValue[struct{}](ctx, policy, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func DoValue[T any](ctx context.Context, policy Policy, fn func() (T, error)) (T, error) {
	p := policy.normalized()
	delay := p.InitialDelay
	var zero T

	for i := 1; i <= p.MaxAttempts; i++ {
		v, err := fn()
		if err == nil {
			return v, nil
		}
		if !backend.IsRetryableError(err) || i == p.MaxAttempts {
			return zero, err
		}

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}

		next := time.Duration(float64(delay) * p.Multiplier)
		if next > p.MaxDelay {
			next = p.MaxDelay
		}
		delay = next
	}
	return zero, nil
}
