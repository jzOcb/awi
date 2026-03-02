package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jzOcb/awi/internal/backend"
)

func retryableErr() error {
	return backend.NewBackendError(backend.ErrUpstream, "test", "op", true, errors.New("temporary"))
}

func permanentErr() error {
	return backend.NewBackendError(backend.ErrParse, "test", "op", false, errors.New("permanent"))
}

var defaultPolicy = Policy{
	MaxAttempts:  3,
	InitialDelay: 1 * time.Millisecond,
	Multiplier:   2,
	MaxDelay:     10 * time.Millisecond,
}

func TestDo_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	err := Do(context.Background(), defaultPolicy, func() error {
		calls++
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("expected success on first call, got err=%v calls=%d", err, calls)
	}
}

func TestDo_RetryThenSuccess(t *testing.T) {
	calls := 0
	err := Do(context.Background(), defaultPolicy, func() error {
		calls++
		if calls < 3 {
			return retryableErr()
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("expected success after 3 calls, got err=%v calls=%d", err, calls)
	}
}

func TestDo_PermanentErrorNoRetry(t *testing.T) {
	calls := 0
	err := Do(context.Background(), defaultPolicy, func() error {
		calls++
		return permanentErr()
	})
	if err == nil || calls != 1 {
		t.Fatalf("expected immediate failure, got err=%v calls=%d", err, calls)
	}
}

func TestDo_ExhaustRetries(t *testing.T) {
	calls := 0
	err := Do(context.Background(), defaultPolicy, func() error {
		calls++
		return retryableErr()
	})
	if err == nil || calls != 3 {
		t.Fatalf("expected exhausted retries, got err=%v calls=%d", err, calls)
	}
}

func TestDo_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := Do(ctx, defaultPolicy, func() error {
		calls++
		cancel()
		return retryableErr()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDoValue_ReturnsValue(t *testing.T) {
	v, err := DoValue[int](context.Background(), defaultPolicy, func() (int, error) {
		return 42, nil
	})
	if err != nil || v != 42 {
		t.Fatalf("expected 42, got %d err=%v", v, err)
	}
}
