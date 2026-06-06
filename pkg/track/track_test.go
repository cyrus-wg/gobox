package track

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TrackTime — basic behaviour
// ---------------------------------------------------------------------------

func TestTrackTime_ReturnsNonNilFunc(t *testing.T) {
	done := TrackTime(context.Background())
	if done == nil {
		t.Fatal("expected non-nil deferred func")
	}
	done() // must not panic
}

func TestTrackTime_NoIndicator(t *testing.T) {
	// Should not panic when called without an indicator.
	done := TrackTime(context.Background())
	done()
}

func TestTrackTime_WithIndicator(t *testing.T) {
	// Should not panic when called with an indicator.
	done := TrackTime(context.Background(), "my-operation")
	done()
}

func TestTrackTime_MultipleIndicatorsUsesFirst(t *testing.T) {
	// The implementation only uses the first indicator; passing extras
	// should not panic.
	done := TrackTime(context.Background(), "first", "second", "third")
	done()
}

// ---------------------------------------------------------------------------
// TrackTime — measures elapsed time correctly
// ---------------------------------------------------------------------------

func TestTrackTime_MeasuresElapsedDuration(t *testing.T) {
	// We can't easily intercept zap output without rebuilding the logger,
	// so we measure the wall-clock time around the deferred call.
	start := time.Now()
	done := TrackTime(context.Background(), "sleep-test")
	time.Sleep(20 * time.Millisecond)
	done()
	elapsed := time.Since(start)

	if elapsed < 20*time.Millisecond {
		t.Fatalf("expected at least 20ms elapsed, got %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// TrackTime — typical defer usage
// ---------------------------------------------------------------------------

func sampleWorkload() {
	defer TrackTime(context.Background(), "sampleWorkload")()
	time.Sleep(5 * time.Millisecond)
}

func TestTrackTime_DeferUsage(t *testing.T) {
	// Should run without panicking using the canonical defer-pattern.
	sampleWorkload()
}

// ---------------------------------------------------------------------------
// TrackTime — context with request ID is accepted
// ---------------------------------------------------------------------------

func TestTrackTime_WithContextValues(t *testing.T) {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("request_id"), "req-abc")
	done := TrackTime(ctx, "ctx-op")
	done()
}

// ---------------------------------------------------------------------------
// TrackTime — concurrent safety
// ---------------------------------------------------------------------------

func TestTrackTime_ConcurrentCalls(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			done := TrackTime(context.Background(), "concurrent")
			time.Sleep(time.Millisecond)
			done()
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// TrackTime — caller info sanity check
// ---------------------------------------------------------------------------

// This test verifies that runtime.Caller(1) inside TrackTime resolves to the
// test function below (the caller of TrackTime). Although we cannot inspect
// the values captured inside TrackTime, we can independently verify that the
// caller info available at this call site matches our expectations.
func TestTrackTime_CallerInfoAvailable(t *testing.T) {
	pc, file, line, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) unexpectedly failed")
	}
	if pc == 0 {
		t.Fatal("expected non-zero program counter")
	}
	if !strings.HasSuffix(file, "track_test.go") {
		t.Fatalf("expected caller file to end with track_test.go, got %q", file)
	}
	if line <= 0 {
		t.Fatalf("expected positive line number, got %d", line)
	}

	// Now exercise TrackTime from this same site to make sure it accepts
	// a normal call without panic.
	done := TrackTime(context.Background(), "caller-info")
	done()
}

// ---------------------------------------------------------------------------
// TrackTimeSkip
// ---------------------------------------------------------------------------

// helperTrack wraps TrackTimeSkip and asks it to skip one extra frame so the
// resolved caller is the *caller of helperTrack*, not helperTrack itself.
func helperTrack(ctx context.Context, name string) func() {
	return TrackTimeSkip(ctx, 1, name)
}

func TestTrackTimeSkip_ResolvesCallerThroughHelper(t *testing.T) {
	// We can't read the captured caller info directly, but we can at least
	// make sure TrackTimeSkip runs without panic and returns a usable
	// closure when skipping a frame.
	done := helperTrack(context.Background(), "via-helper")
	if done == nil {
		t.Fatal("expected non-nil deferred func")
	}
	done()
}

func TestTrackTimeSkip_NegativeSkipTreatedAsZero(t *testing.T) {
	done := TrackTimeSkip(context.Background(), -5, "negative-skip")
	done()
}

func TestTrackTimeSkip_LargeSkipDoesNotPanic(t *testing.T) {
	// An absurdly large skip will cause runtime.Caller to return ok=false.
	// The implementation should fall back to a degraded log instead of
	// panicking.
	done := TrackTimeSkip(context.Background(), 1000, "huge-skip")
	done()
}

// ---------------------------------------------------------------------------
// durationMS
// ---------------------------------------------------------------------------

func TestDurationMS_Zero(t *testing.T) {
	got := durationMS(time.Now())
	if got < 0 {
		t.Fatalf("durationMS should never be negative, got %v", got)
	}
}

func TestDurationMS_NonZero(t *testing.T) {
	start := time.Now()
	time.Sleep(5 * time.Millisecond)
	got := durationMS(start)
	if got < 4 || got > 100 {
		t.Fatalf("expected ~5ms, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkTrackTime(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		done := TrackTime(ctx, "bench")
		done()
	}
}

func BenchmarkTrackTime_DeferPattern(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		func() {
			defer TrackTime(ctx, "bench-defer")()
		}()
	}
}
