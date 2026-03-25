package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cyrus-wg/gobox/pkg/retry"
)

var errTemporary = errors.New("temporary failure")

func main() {
	fmt.Println("=== retry package examples ===")
	fmt.Println()

	ctx := context.Background()

	// --- Do: returns a value ---
	fmt.Println("// Do — succeeds on 3rd attempt")
	attempt := 0
	val, err := retry.Do(ctx, func(ctx context.Context) (string, error) {
		attempt++
		fmt.Printf("  attempt %d\n", attempt)
		if attempt < 3 {
			return "", errTemporary
		}
		return "success", nil
	}, retry.WithMaxAttempts(5), retry.WithInitialDelay(50*time.Millisecond))
	fmt.Printf("  result=%q err=%v\n\n", val, err)

	// --- Run: error-only function ---
	fmt.Println("// Run — always fails, exhausts attempts")
	err = retry.Run(ctx, func(ctx context.Context) error {
		return errTemporary
	}, retry.WithMaxAttempts(3), retry.WithInitialDelay(10*time.Millisecond))
	fmt.Printf("  err=%v\n\n", err)

	// --- Constant delay ---
	fmt.Println("// Run — constant 20ms delay")
	start := time.Now()
	attempt = 0
	err = retry.Run(ctx, func(ctx context.Context) error {
		attempt++
		return errTemporary
	}, retry.WithMaxAttempts(4), retry.WithConstantDelay(20*time.Millisecond))
	fmt.Printf("  attempts=%d elapsed=%v err=%v\n\n", attempt, time.Since(start).Round(time.Millisecond), err)

	// --- Custom multiplier ---
	fmt.Println("// Run — custom multiplier (3x growth)")
	start = time.Now()
	attempt = 0
	err = retry.Run(ctx, func(ctx context.Context) error {
		attempt++
		return errTemporary
	}, retry.WithMaxAttempts(4), retry.WithInitialDelay(10*time.Millisecond), retry.WithMultiplier(3.0))
	fmt.Printf("  attempts=%d elapsed=%v err=%v\n\n", attempt, time.Since(start).Round(time.Millisecond), err)

	// --- MaxDelay caps exponential growth ---
	fmt.Println("// Run — MaxDelay caps backoff at 25ms")
	start = time.Now()
	attempt = 0
	err = retry.Run(ctx, func(ctx context.Context) error {
		attempt++
		return errTemporary
	}, retry.WithMaxAttempts(5), retry.WithInitialDelay(10*time.Millisecond), retry.WithMultiplier(2.0), retry.WithMaxDelay(25*time.Millisecond))
	fmt.Printf("  attempts=%d elapsed=%v err=%v\n\n", attempt, time.Since(start).Round(time.Millisecond), err)

	// --- Exponential backoff with jitter ---
	fmt.Println("// Run — exponential backoff with jitter")
	start = time.Now()
	attempt = 0
	err = retry.Run(ctx, func(ctx context.Context) error {
		attempt++
		return errTemporary
	}, retry.WithMaxAttempts(4), retry.WithInitialDelay(10*time.Millisecond), retry.WithJitter(true))
	fmt.Printf("  attempts=%d elapsed=%v err=%v\n\n", attempt, time.Since(start).Round(time.Millisecond), err)

	// --- WithOnRetry: logging callback ---
	fmt.Println("// Run — WithOnRetry callback for logging")
	attempt = 0
	_ = retry.Run(ctx, func(ctx context.Context) error {
		attempt++
		return errTemporary
	}, retry.WithMaxAttempts(3), retry.WithInitialDelay(10*time.Millisecond), retry.WithOnRetry(func(ctx context.Context, attempt int, err error) {
		fmt.Printf("  [OnRetry] attempt %d failed: %v\n", attempt, err)
	}))
	fmt.Println()

	// --- WithRetryIf: skip non-retryable errors ---
	fmt.Println("// Run — non-retryable error stops immediately")
	permanent := errors.New("permanent failure")
	attempt = 0
	err = retry.Run(ctx, func(ctx context.Context) error {
		attempt++
		fmt.Printf("  attempt %d\n", attempt)
		return permanent
	}, retry.WithMaxAttempts(5), retry.WithInitialDelay(time.Millisecond), retry.WithRetryIf(func(err error) bool {
		return !errors.Is(err, permanent)
	}))
	fmt.Printf("  attempts=%d err=%v\n\n", attempt, err)

	// --- Context cancellation ---
	fmt.Println("// Run — context timeout stops retries")
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	attempt = 0
	err = retry.Run(timeoutCtx, func(ctx context.Context) error {
		attempt++
		return errTemporary
	}, retry.WithMaxAttempts(100), retry.WithInitialDelay(20*time.Millisecond))
	fmt.Printf("  attempts=%d err=%v\n", attempt, err)
}
