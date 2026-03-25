package retry

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

const (
	defaultMaxAttempts  = 3
	defaultInitialDelay = 100 * time.Millisecond
	defaultMaxDelay     = 10 * time.Second
	defaultMultiplier   = 2.0
)

// ---------------------------------------------------------------------------
// Config / Options
// ---------------------------------------------------------------------------

// Config controls retry behaviour. Use functional options (WithXxx) with
// Do or Run; zero values are replaced with safe defaults.
type Config struct {
	// MaxAttempts is the total number of calls including the initial one.
	// Values ≤ 0 default to 3.
	MaxAttempts int

	// InitialDelay is the wait time before the first retry.
	// Values ≤ 0 default to 100 ms.
	InitialDelay time.Duration

	// MaxDelay caps the computed delay between retries.
	// Values ≤ 0 default to 10 s.
	MaxDelay time.Duration

	// Multiplier controls exponential growth of the delay.
	// A multiplier of 1.0 gives constant delay. Values ≤ 0 default to 2.0.
	Multiplier float64

	// Jitter randomises the delay (uniform [0, computed delay]) to avoid
	// thundering-herd problems when many callers retry simultaneously.
	Jitter bool

	// IsRetryable determines whether a given error should be retried.
	// If nil, all non-nil errors are considered retryable.
	IsRetryable func(error) bool

	// OnRetry is called after each failed attempt, before the delay/sleep.
	// Receives the context, the 1-based attempt number that just failed,
	// and the error returned by that attempt.
	// If nil (the default), no callback is invoked.
	OnRetry func(ctx context.Context, attempt int, err error)
}

// Option configures retry behaviour.
type Option func(*Config)

// WithMaxAttempts sets the total number of calls (initial + retries).
//
//	retry.Do(ctx, fn, retry.WithMaxAttempts(5)) // 1 initial + 4 retries
func WithMaxAttempts(n int) Option {
	return func(c *Config) { c.MaxAttempts = n }
}

// WithInitialDelay sets the delay before the first retry.
func WithInitialDelay(d time.Duration) Option {
	return func(c *Config) { c.InitialDelay = d }
}

// WithMaxDelay caps the delay between retries.
func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) { c.MaxDelay = d }
}

// WithMultiplier sets the exponential backoff multiplier applied to the
// delay after each failed attempt. A multiplier of 1.0 produces constant
// delay; 2.0 doubles the delay each time.
func WithMultiplier(m float64) Option {
	return func(c *Config) { c.Multiplier = m }
}

// WithJitter enables random jitter on the computed delay. When enabled, the
// actual sleep is uniformly distributed in [0, computed delay].
func WithJitter(enabled bool) Option {
	return func(c *Config) { c.Jitter = enabled }
}

// WithConstantDelay is a convenience option that sets the delay to d with
// no exponential growth (multiplier = 1.0).
//
//	retry.Run(ctx, fn, retry.WithConstantDelay(500*time.Millisecond))
func WithConstantDelay(d time.Duration) Option {
	return func(c *Config) {
		c.InitialDelay = d
		c.Multiplier = 1.0
	}
}

// WithRetryIf provides a predicate that decides whether an error is
// retryable. When the predicate returns false the retry loop stops
// immediately and the error is returned.
// A nil predicate (the default) retries all non-nil errors.
//
//	retry.Run(ctx, fn, retry.WithRetryIf(func(err error) bool {
//	    return errors.Is(err, io.ErrUnexpectedEOF)
//	}))
func WithRetryIf(fn func(error) bool) Option {
	return func(c *Config) { c.IsRetryable = fn }
}

// WithOnRetry registers a callback invoked after each failed attempt,
// before the backoff delay.
// The attempt parameter is 1-based (attempt 1 is the first call).
//
//	retry.Run(ctx, fn, retry.WithOnRetry(func(ctx context.Context, attempt int, err error) {
//	    logger.Warnf(ctx, "retry attempt %d failed: %v", attempt, err)
//	}))
func WithOnRetry(fn func(ctx context.Context, attempt int, err error)) Option {
	return func(c *Config) { c.OnRetry = fn }
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Do executes fn, retrying on error according to the provided options.
// The context is passed to fn on every attempt and is checked between
// retries — if the context is cancelled the retry loop stops and the
// context error is returned.
//
// Returns the first successful result, or the last error after all
// attempts are exhausted.
//
//	val, err := retry.Do(ctx, func(ctx context.Context) (int, error) {
//	    return fetchFromAPI(ctx)
//	}, retry.WithMaxAttempts(5), retry.WithJitter(true))
func Do[T any](ctx context.Context, fn func(ctx context.Context) (T, error), opts ...Option) (T, error) {
	cfg := buildConfig(opts)

	var zero T
	var lastErr error

	for attempt := range cfg.MaxAttempts {
		if attempt > 0 {
			delay := computeDelay(cfg, attempt)
			if err := sleep(ctx, delay); err != nil {
				return zero, err
			}
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err

		if cfg.OnRetry != nil {
			cfg.OnRetry(ctx, attempt+1, err)
		}

		if cfg.IsRetryable != nil && !cfg.IsRetryable(err) {
			return zero, err
		}
	}

	return zero, lastErr
}

// Run is like Do but for functions that return only an error.
//
//	err := retry.Run(ctx, func(ctx context.Context) error {
//	    return sendWebhook(ctx, payload)
//	}, retry.WithMaxAttempts(3))
func Run(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error {
	_, err := Do(ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	}, opts...)
	return err
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func buildConfig(opts []Option) Config {
	cfg := Config{
		MaxAttempts:  defaultMaxAttempts,
		InitialDelay: defaultInitialDelay,
		MaxDelay:     defaultMaxDelay,
		Multiplier:   defaultMultiplier,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaultMaxAttempts
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = defaultInitialDelay
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = defaultMaxDelay
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = defaultMultiplier
	}
	return cfg
}

// computeDelay returns the delay for the given attempt number (1-indexed:
// attempt 1 is the first retry). The result is capped by MaxDelay and
// optionally jittered.
func computeDelay(cfg Config, attempt int) time.Duration {
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	if cfg.Jitter {
		delay = rand.Float64() * delay
	}
	return time.Duration(delay)
}

// sleep blocks for duration d or until ctx is done, whichever comes first.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
