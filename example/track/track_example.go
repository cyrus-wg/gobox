package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"github.com/cyrus-wg/gobox/pkg/track"
)

// fastWork demonstrates the canonical defer-pattern with TrackTime.
func fastWork(ctx context.Context) {
	defer track.TrackTime(ctx, "fastWork")()
	time.Sleep(10 * time.Millisecond)
}

// slowWork demonstrates tracking a longer running function.
func slowWork(ctx context.Context) {
	defer track.TrackTime(ctx, "slowWork")()
	time.Sleep(75 * time.Millisecond)
}

// noIndicator demonstrates TrackTime without an indicator label.
func noIndicator(ctx context.Context) {
	defer track.TrackTime(ctx)()
	time.Sleep(15 * time.Millisecond)
}

// manualStop demonstrates calling the returned func explicitly instead
// of using `defer`. This is useful when you only want to measure a
// portion of a function.
func manualStop(ctx context.Context) {
	// Some setup work that we don't want to track.
	time.Sleep(5 * time.Millisecond)

	stop := track.TrackTime(ctx, "manualStop:critical-section")
	// Critical section we DO want to track.
	time.Sleep(30 * time.Millisecond)
	stop()

	// Some teardown work we don't want to track.
	time.Sleep(5 * time.Millisecond)
}

// nestedTracking shows how multiple TrackTime calls can coexist in a single
// function, each independently measuring its own scope.
func nestedTracking(ctx context.Context) {
	defer track.TrackTime(ctx, "nestedTracking:outer")()

	func() {
		defer track.TrackTime(ctx, "nestedTracking:inner-1")()
		time.Sleep(10 * time.Millisecond)
	}()

	func() {
		defer track.TrackTime(ctx, "nestedTracking:inner-2")()
		time.Sleep(20 * time.Millisecond)
	}()
}

// trackedHelper is a thin wrapper around TrackTimeSkip. By skipping one
// extra frame, the log entry reports the *caller of trackedHelper* as the
// tracked location — not trackedHelper itself.
func trackedHelper(ctx context.Context, name string) func() {
	return track.TrackTimeSkip(ctx, 1, name)
}

// viaHelper demonstrates how TrackTimeSkip preserves caller attribution when
// TrackTime is hidden behind a helper.
func viaHelper(ctx context.Context) {
	defer trackedHelper(ctx, "viaHelper")()
	time.Sleep(8 * time.Millisecond)
}

func main() {
	// Make sure logs are visible on stderr.
	logger.InitGlobalLogger(logger.LoggerConfig{DebugLogLevel: true})

	// Attach a request ID so the tracked log entries are correlated.
	ctx := logger.SetRequestID(context.Background(), logger.GenerateRequestID())

	fmt.Println("=== track package examples ===")
	fmt.Println()

	fmt.Println("// fastWork — short operation tracked with defer")
	fastWork(ctx)
	fmt.Println()

	fmt.Println("// slowWork — longer operation tracked with defer")
	slowWork(ctx)
	fmt.Println()

	fmt.Println("// noIndicator — TrackTime called without an indicator label")
	noIndicator(ctx)
	fmt.Println()

	fmt.Println("// manualStop — explicitly stop tracking mid-function")
	manualStop(ctx)
	fmt.Println()

	fmt.Println("// nestedTracking — multiple overlapping scopes")
	nestedTracking(ctx)
	fmt.Println()

	fmt.Println("// viaHelper — TrackTimeSkip attributes the call to viaHelper, not the helper wrapper")
	viaHelper(ctx)
	fmt.Println()

	// Ensure all buffered log lines are flushed before exit.
	logger.Flush()
}
