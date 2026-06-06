// Package track provides lightweight utilities for measuring and logging the
// wall-clock duration of a code block.
//
// The canonical usage is the defer-pattern:
//
//	func DoWork(ctx context.Context) {
//	    defer track.TrackTime(ctx, "DoWork")()
//	    // ... work ...
//	}
//
// When the deferred function fires, an Info-level structured log entry is
// emitted with the caller's file / function / line, the start timestamp, the
// elapsed duration in milliseconds, and the optional indicator label.
package track

import (
	"context"
	"runtime"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
)

// TrackTime starts a timer and returns a function that, when invoked, emits a
// structured log entry containing the elapsed duration since TrackTime was
// called. Only the first element of indicator is used; additional elements are
// ignored.
//
// runtime.Caller is invoked once on entry to resolve the caller's location,
// so the cost (a few hundred ns) is paid once per call — not on every log.
func TrackTime(ctx context.Context, indicator ...string) func() {
	// One frame to skip: TrackTime itself. trackTimeAt adds one more for
	// its own frame.
	return trackTimeAt(ctx, 1, indicator...)
}

// TrackTimeSkip behaves like TrackTime but lets the caller skip additional
// stack frames when resolving the caller location. Use this when TrackTime is
// invoked from inside a helper/wrapper and you want the log to reference the
// helper's caller instead of the helper itself.
//
//	func tracked(ctx context.Context, name string) func() {
//	    return track.TrackTimeSkip(ctx, 1, name) // skip `tracked` too
//	}
func TrackTimeSkip(ctx context.Context, skip int, indicator ...string) func() {
	if skip < 0 {
		skip = 0
	}
	return trackTimeAt(ctx, skip+1, indicator...)
}

// trackTimeAt is the shared implementation. skip is the number of stack frames
// to skip *above* trackTimeAt (i.e. skip=1 means the immediate caller of
// trackTimeAt, skip=2 means that caller's caller, ...).
func trackTimeAt(ctx context.Context, skip int, indicator ...string) func() {
	start := time.Now()

	idc := ""
	if len(indicator) > 0 {
		idc = indicator[0]
	}

	// +1 to skip trackTimeAt's own frame.
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		logger.Warn(ctx, "track: failed to resolve runtime caller")
		// Still log the duration — the user explicitly asked to track
		// something, so a missing caller shouldn't silently drop it.
		return func() {
			logger.Infow(ctx, "Track time",
				"indicator", idc,
				"start", start,
				"duration_ms", durationMS(start),
			)
		}
	}

	// Resolve the function name eagerly so we don't repeat the work inside
	// the deferred closure (and so we can guard against a nil *Func).
	fnName := "unknown"
	if fn := runtime.FuncForPC(pc); fn != nil {
		fnName = fn.Name()
	}

	return func() {
		logger.Infow(ctx, "Track time",
			"indicator", idc,
			"file", file,
			"function", fnName,
			"line", line,
			"start", start,
			"duration_ms", durationMS(start),
		)
	}
}

// durationMS returns the elapsed time since start in milliseconds, preserving
// sub-millisecond precision.
func durationMS(start time.Time) float64 {
	return float64(time.Since(start)) / float64(time.Millisecond)
}
