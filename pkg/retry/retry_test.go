package retry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

var errTemporary = errors.New("temporary failure")

// ---------------------------------------------------------------------------
// Do — success cases
// ---------------------------------------------------------------------------

func TestDo_SucceedsFirstAttempt(t *testing.T) {
	calls := 0
	val, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		calls++
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDo_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	val, err := Do(context.Background(), func(ctx context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "", errTemporary
		}
		return "ok", nil
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "ok" {
		t.Fatalf("expected ok, got %q", val)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

// ---------------------------------------------------------------------------
// Do — exhausted attempts
// ---------------------------------------------------------------------------

func TestDo_ExhaustsAllAttempts(t *testing.T) {
	calls := 0
	_, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		calls++
		return 0, errTemporary
	}, WithMaxAttempts(4), WithInitialDelay(time.Millisecond))
	if !errors.Is(err, errTemporary) {
		t.Fatalf("expected errTemporary, got %v", err)
	}
	if calls != 4 {
		t.Fatalf("expected 4 calls, got %d", calls)
	}
}

func TestDo_SingleAttempt(t *testing.T) {
	calls := 0
	_, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		calls++
		return 0, errTemporary
	}, WithMaxAttempts(1), WithInitialDelay(time.Millisecond))
	if !errors.Is(err, errTemporary) {
		t.Fatalf("expected errTemporary, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

func TestRun_Success(t *testing.T) {
	calls := 0
	err := Run(context.Background(), func(ctx context.Context) error {
		calls++
		if calls < 2 {
			return errTemporary
		}
		return nil
	}, WithMaxAttempts(3), WithInitialDelay(time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestRun_Exhausted(t *testing.T) {
	calls := 0
	err := Run(context.Background(), func(ctx context.Context) error {
		calls++
		return errTemporary
	}, WithMaxAttempts(2), WithInitialDelay(time.Millisecond))
	if !errors.Is(err, errTemporary) {
		t.Fatalf("expected errTemporary, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := Do(ctx, func(ctx context.Context) (int, error) {
		calls++
		cancel() // cancel after first attempt
		return 0, errTemporary
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call before cancel, got %d", calls)
	}
}

func TestDo_ContextDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	calls := 0
	_, err := Do(ctx, func(ctx context.Context) (int, error) {
		calls++
		return 0, errTemporary
	}, WithMaxAttempts(100), WithInitialDelay(50*time.Millisecond))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// WithRetryIf — non-retryable errors
// ---------------------------------------------------------------------------

func TestDo_NonRetryableError(t *testing.T) {
	permanent := errors.New("permanent")
	calls := 0
	_, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		calls++
		return 0, permanent
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond), WithRetryIf(func(err error) bool {
		return !errors.Is(err, permanent)
	}))
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for non-retryable), got %d", calls)
	}
}

func TestDo_RetryIfAllowsRetry(t *testing.T) {
	calls := 0
	_, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		calls++
		return 0, errTemporary
	}, WithMaxAttempts(3), WithInitialDelay(time.Millisecond), WithRetryIf(func(err error) bool {
		return errors.Is(err, errTemporary)
	}))
	if !errors.Is(err, errTemporary) {
		t.Fatalf("expected errTemporary, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

// ---------------------------------------------------------------------------
// WithConstantDelay
// ---------------------------------------------------------------------------

func TestDo_ConstantDelay(t *testing.T) {
	start := time.Now()
	calls := 0
	_ = Run(context.Background(), func(ctx context.Context) error {
		calls++
		return errTemporary
	}, WithMaxAttempts(4), WithConstantDelay(10*time.Millisecond))

	elapsed := time.Since(start)
	if calls != 4 {
		t.Fatalf("expected 4 calls, got %d", calls)
	}
	// 3 delays × 10ms = ~30ms minimum
	if elapsed < 25*time.Millisecond {
		t.Fatalf("expected at least ~30ms, got %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// WithJitter
// ---------------------------------------------------------------------------

func TestDo_WithJitter(t *testing.T) {
	// Jitter should not cause errors or panics; delay should be ≤ computed delay.
	calls := 0
	_ = Run(context.Background(), func(ctx context.Context) error {
		calls++
		return errTemporary
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond), WithJitter(true))
	if calls != 5 {
		t.Fatalf("expected 5 calls, got %d", calls)
	}
}

// ---------------------------------------------------------------------------
// Exponential backoff timing
// ---------------------------------------------------------------------------

func TestDo_ExponentialBackoff(t *testing.T) {
	// 3 retries with 20ms initial, multiplier 2 → delays: 20ms, 40ms, 80ms = ~140ms total
	start := time.Now()
	_ = Run(context.Background(), func(ctx context.Context) error {
		return errTemporary
	}, WithMaxAttempts(4), WithInitialDelay(20*time.Millisecond), WithMultiplier(2.0))

	elapsed := time.Since(start)
	if elapsed < 120*time.Millisecond {
		t.Fatalf("expected at least ~140ms of backoff, got %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// MaxDelay cap
// ---------------------------------------------------------------------------

func TestDo_MaxDelayCapsBackoff(t *testing.T) {
	// Without cap: 10ms, 20ms, 40ms = 70ms.
	// With MaxDelay=15ms: 10ms, 15ms, 15ms = 40ms.
	start := time.Now()
	_ = Run(context.Background(), func(ctx context.Context) error {
		return errTemporary
	}, WithMaxAttempts(4), WithInitialDelay(10*time.Millisecond), WithMultiplier(2.0), WithMaxDelay(15*time.Millisecond))

	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("MaxDelay should cap backoff, but took %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// buildConfig defaults
// ---------------------------------------------------------------------------

func TestBuildConfig_Defaults(t *testing.T) {
	cfg := buildConfig(nil)
	if cfg.MaxAttempts != 3 {
		t.Fatalf("expected 3, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelay != 100*time.Millisecond {
		t.Fatalf("expected 100ms, got %v", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 10*time.Second {
		t.Fatalf("expected 10s, got %v", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Fatalf("expected 2.0, got %f", cfg.Multiplier)
	}
	if cfg.Jitter {
		t.Fatal("expected jitter off by default")
	}
	if cfg.IsRetryable != nil {
		t.Fatal("expected nil IsRetryable by default")
	}
}

func TestBuildConfig_NegativeValuesGetDefaults(t *testing.T) {
	cfg := buildConfig([]Option{
		WithMaxAttempts(-1),
		WithInitialDelay(-5 * time.Second),
		WithMaxDelay(-1 * time.Second),
		WithMultiplier(-0.5),
	})
	if cfg.MaxAttempts != defaultMaxAttempts {
		t.Fatalf("expected default %d, got %d", defaultMaxAttempts, cfg.MaxAttempts)
	}
	if cfg.InitialDelay != defaultInitialDelay {
		t.Fatalf("expected default %v, got %v", defaultInitialDelay, cfg.InitialDelay)
	}
	if cfg.MaxDelay != defaultMaxDelay {
		t.Fatalf("expected default %v, got %v", defaultMaxDelay, cfg.MaxDelay)
	}
	if cfg.Multiplier != defaultMultiplier {
		t.Fatalf("expected default %f, got %f", defaultMultiplier, cfg.Multiplier)
	}
}

func TestBuildConfig_PreservesExplicitValues(t *testing.T) {
	cfg := buildConfig([]Option{
		WithMaxAttempts(10),
		WithInitialDelay(500 * time.Millisecond),
		WithMaxDelay(30 * time.Second),
		WithMultiplier(3.0),
		WithJitter(true),
	})
	if cfg.MaxAttempts != 10 {
		t.Fatalf("expected 10, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelay != 500*time.Millisecond {
		t.Fatalf("expected 500ms, got %v", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Fatalf("expected 30s, got %v", cfg.MaxDelay)
	}
	if cfg.Multiplier != 3.0 {
		t.Fatalf("expected 3.0, got %f", cfg.Multiplier)
	}
	if !cfg.Jitter {
		t.Fatal("expected jitter on")
	}
}

// ---------------------------------------------------------------------------
// computeDelay
// ---------------------------------------------------------------------------

func TestComputeDelay_Exponential(t *testing.T) {
	cfg := Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 100 * time.Millisecond},  // 100 * 2^0
		{2, 200 * time.Millisecond},  // 100 * 2^1
		{3, 400 * time.Millisecond},  // 100 * 2^2
		{4, 800 * time.Millisecond},  // 100 * 2^3
		{5, 1600 * time.Millisecond}, // 100 * 2^4
	}
	for _, tt := range tests {
		got := computeDelay(cfg, tt.attempt)
		if got != tt.want {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.want, got)
		}
	}
}

func TestComputeDelay_CappedByMaxDelay(t *testing.T) {
	cfg := Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     300 * time.Millisecond,
		Multiplier:   2.0,
	}
	// attempt 3 → 400ms uncapped, should be capped to 300ms
	got := computeDelay(cfg, 3)
	if got != 300*time.Millisecond {
		t.Fatalf("expected 300ms cap, got %v", got)
	}
}

func TestComputeDelay_ConstantMultiplier(t *testing.T) {
	cfg := Config{
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   1.0,
	}
	for attempt := 1; attempt <= 5; attempt++ {
		got := computeDelay(cfg, attempt)
		if got != 50*time.Millisecond {
			t.Errorf("attempt %d: expected 50ms (constant), got %v", attempt, got)
		}
	}
}

func TestComputeDelay_WithJitter(t *testing.T) {
	cfg := Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
	// Run many times — all results should be in [0, 100ms]
	for i := 0; i < 100; i++ {
		got := computeDelay(cfg, 1)
		if got < 0 || got > 100*time.Millisecond {
			t.Fatalf("jittered delay out of range: %v", got)
		}
	}
}

// ---------------------------------------------------------------------------
// sleep
// ---------------------------------------------------------------------------

func TestSleep_Completes(t *testing.T) {
	start := time.Now()
	err := sleep(context.Background(), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) < 8*time.Millisecond {
		t.Fatal("sleep returned too early")
	}
}

func TestSleep_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sleep(ctx, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestSleep_ZeroDuration(t *testing.T) {
	err := sleep(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// WithOnRetry
// ---------------------------------------------------------------------------

func TestDo_OnRetryCalledOnFailure(t *testing.T) {
	var recorded []int
	var recordedErrs []error
	_, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		return 0, errTemporary
	}, WithMaxAttempts(3), WithInitialDelay(time.Millisecond), WithOnRetry(func(ctx context.Context, attempt int, err error) {
		recorded = append(recorded, attempt)
		recordedErrs = append(recordedErrs, err)
	}))
	if !errors.Is(err, errTemporary) {
		t.Fatalf("expected errTemporary, got %v", err)
	}
	if len(recorded) != 3 {
		t.Fatalf("expected 3 OnRetry calls, got %d", len(recorded))
	}
	// Attempts should be 1-based: 1, 2, 3
	for i, want := range []int{1, 2, 3} {
		if recorded[i] != want {
			t.Errorf("OnRetry call %d: expected attempt %d, got %d", i, want, recorded[i])
		}
		if !errors.Is(recordedErrs[i], errTemporary) {
			t.Errorf("OnRetry call %d: expected errTemporary, got %v", i, recordedErrs[i])
		}
	}
}

func TestDo_OnRetryNotCalledOnSuccess(t *testing.T) {
	called := false
	val, err := Do(context.Background(), func(ctx context.Context) (string, error) {
		return "ok", nil
	}, WithOnRetry(func(ctx context.Context, attempt int, err error) {
		called = true
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "ok" {
		t.Fatalf("expected ok, got %q", val)
	}
	if called {
		t.Fatal("OnRetry should not be called when fn succeeds")
	}
}

func TestDo_OnRetryCalledBeforeNonRetryableExit(t *testing.T) {
	permanent := errors.New("permanent")
	var recorded []int
	_, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		return 0, permanent
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond), WithRetryIf(func(err error) bool {
		return false // nothing is retryable
	}), WithOnRetry(func(ctx context.Context, attempt int, err error) {
		recorded = append(recorded, attempt)
	}))
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent, got %v", err)
	}
	// OnRetry should still be called once for the single failed attempt
	if len(recorded) != 1 || recorded[0] != 1 {
		t.Fatalf("expected [1], got %v", recorded)
	}
}

func TestDo_OnRetryReceivesContext(t *testing.T) {
	type key string
	ctx := context.WithValue(context.Background(), key("id"), "test-123")
	var capturedID string
	_ = Run(ctx, func(ctx context.Context) error {
		return errTemporary
	}, WithMaxAttempts(2), WithInitialDelay(time.Millisecond), WithOnRetry(func(ctx context.Context, attempt int, err error) {
		capturedID, _ = ctx.Value(key("id")).(string)
	}))
	if capturedID != "test-123" {
		t.Fatalf("expected test-123, got %q", capturedID)
	}
}

func TestRun_OnRetryWorks(t *testing.T) {
	var recorded []int
	_ = Run(context.Background(), func(ctx context.Context) error {
		return errTemporary
	}, WithMaxAttempts(2), WithInitialDelay(time.Millisecond), WithOnRetry(func(ctx context.Context, attempt int, err error) {
		recorded = append(recorded, attempt)
	}))
	if len(recorded) != 2 {
		t.Fatalf("expected 2 OnRetry calls, got %d", len(recorded))
	}
}

func TestDo_OnRetryNilIsNoOp(t *testing.T) {
	// Verify nil OnRetry (default) doesn't panic
	_, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		return 0, errTemporary
	}, WithMaxAttempts(2), WithInitialDelay(time.Millisecond))
	if !errors.Is(err, errTemporary) {
		t.Fatalf("expected errTemporary, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

func TestDo_ConcurrentCalls(t *testing.T) {
	var total atomic.Int32
	done := make(chan struct{}, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = Do(context.Background(), func(ctx context.Context) (int, error) {
				total.Add(1)
				return 1, nil
			})
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if total.Load() != 10 {
		t.Fatalf("expected 10 calls, got %d", total.Load())
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestDo_SucceedsOnLastAttempt(t *testing.T) {
	calls := 0
	val, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		calls++
		if calls < 5 {
			return 0, errTemporary
		}
		return 99, nil
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 99 {
		t.Fatalf("expected 99, got %d", val)
	}
	if calls != 5 {
		t.Fatalf("expected 5 calls, got %d", calls)
	}
}

func TestDo_ErrorChangesToNonRetryable(t *testing.T) {
	temporary := errors.New("temporary")
	permanent := errors.New("permanent")
	calls := 0
	_, err := Do(context.Background(), func(ctx context.Context) (int, error) {
		calls++
		if calls < 3 {
			return 0, temporary
		}
		return 0, permanent // switches to non-retryable on 3rd call
	}, WithMaxAttempts(10), WithInitialDelay(time.Millisecond), WithRetryIf(func(err error) bool {
		return errors.Is(err, temporary)
	}))
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDo_ReturnsZeroValueOnError(t *testing.T) {
	type result struct {
		Name  string
		Value int
	}
	val, err := Do(context.Background(), func(ctx context.Context) (result, error) {
		return result{}, errTemporary
	}, WithMaxAttempts(1), WithInitialDelay(time.Millisecond))
	if !errors.Is(err, errTemporary) {
		t.Fatalf("expected errTemporary, got %v", err)
	}
	if val != (result{}) {
		t.Fatalf("expected zero value, got %+v", val)
	}
}

func TestRun_WithRetryIf(t *testing.T) {
	permanent := errors.New("permanent")
	calls := 0
	err := Run(context.Background(), func(ctx context.Context) error {
		calls++
		return permanent
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond), WithRetryIf(func(err error) bool {
		return !errors.Is(err, permanent)
	}))
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestSleep_NegativeDuration(t *testing.T) {
	err := sleep(context.Background(), -1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
