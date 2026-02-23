package main

import (
	"context"
	"fmt"

	"github.com/cyrus-wg/gobox/pkg/logger"
)

func main() {
	globalLoggerExample()
	instanceLoggerExample()
}

// globalLoggerExample demonstrates the package-level global logger.
//
// The global logger is initialised automatically with default settings via
// init(), so it can be used immediately without any setup. Call
// logger.InitGlobalLogger to override the defaults before your first log call.
func globalLoggerExample() {
	printSeparator("GLOBAL LOGGER")

	// --- Default usage (no initialisation required) ---
	ctx := context.Background()

	// Attach a request ID so every log line carries it automatically.
	ctx = logger.SetRequestID(ctx, logger.GenerateRequestID())

	reqID, _ := logger.GetRequestID(ctx)
	fmt.Println("request_id:", reqID)

	// Basic log levels (Debug is suppressed at the default INFO level).
	logger.Info(ctx, "server started")
	logger.Warn(ctx, "disk usage above 80%")
	logger.Error(ctx, "failed to connect to database")

	// --- Re-initialise with custom config ---
	// InitGlobalLogger replaces the singleton instance. Call this once at
	// program startup, before spawning goroutines that log.
	logger.InitGlobalLogger(logger.LoggerConfig{
		DebugLogLevel:   true,      // enable DEBUG level
		RequestIDPrefix: "global-", // prefix prepended to generated IDs
		FixedKeyValues: map[string]any{ // key-value pairs added to every log line
			"app":     "gobox-example",
			"version": "1.0.0",
		},
	})

	ctx = context.Background()
	ctx = logger.SetRequestID(ctx, logger.GenerateRequestID())

	reqID, _ = logger.GetRequestID(ctx)
	fmt.Println("request_id:", reqID)

	// All log levels are now available.
	logger.Debug(ctx, "debug is now enabled")
	logger.Info(ctx, "plain message – args are concatenated like fmt.Sprint")

	// Formatted variants mirror fmt.Sprintf.
	logger.Infof(ctx, "processed %d records in %s", 42, "3ms")
	logger.Warnf(ctx, "retry attempt %d of %d", 1, 3)

	// Structured variants accept alternating key-value pairs.
	logger.Infow(ctx, "user action", "user_id", 99, "action", "login")
	logger.Errorw(ctx, "request failed", "status_code", 500, "path", "/api/data")

	// Flush ensures buffered log lines are written before the process exits.
	logger.Flush()
}

// instanceLoggerExample demonstrates creating and using a *logger.Logger
// instance directly.
//
// Use an instance logger when different parts of your application need their
// own configuration (e.g. different fixed fields or log levels).
func instanceLoggerExample() {
	printSeparator("INSTANCE LOGGER")

	// --- Create a logger instance ---
	log := logger.NewLogger(logger.LoggerConfig{
		DebugLogLevel:   true,
		RequestIDPrefix: "svc-",
		FixedKeyValues: map[string]any{
			"service": "payment",
			"region":  "ap-east-1",
		},
		// ExtraFields lists context keys whose values are automatically
		// pulled from the context and appended to every log line.
		ExtraFields: []string{"tenant_id", "trace_id"},
	})

	// --- Attach a request ID ---
	ctx := context.Background()
	ctx = log.SetRequestID(ctx, log.GenerateRequestID())

	reqID, _ := log.GetRequestID(ctx)
	fmt.Println("request_id:", reqID)

	// --- Inject extra context fields ---
	// These match the keys declared in ExtraFields above.
	ctx = context.WithValue(ctx, "tenant_id", "tenant-abc")
	ctx = context.WithValue(ctx, "trace_id", "trace-xyz")

	// --- Log at every level ---
	log.Debug(ctx, "processing payment")
	log.Info(ctx, "payment accepted")
	log.Warn(ctx, "payment gateway latency high")
	log.Error(ctx, "payment gateway unreachable")

	// Formatted variants.
	log.Debugf(ctx, "amount: %.2f %s", 99.95, "HKD")
	log.Infof(ctx, "order #%d confirmed", 1001)

	// Structured variants with ad-hoc key-value pairs.
	log.Infow(ctx, "charge complete", "amount", 99.95, "currency", "HKD", "tx_id", "tx-001")
	log.Errorw(ctx, "charge failed", "reason", "insufficient funds", "tx_id", "tx-002")

	// --- Inspect config at runtime ---
	fmt.Println("debug enabled  :", log.IsDebugLogLevel())
	fmt.Println("request prefix :", log.GetRequestIDPrefix())
	fmt.Println("fixed fields   :", log.GetFixedKeyValues())
	fmt.Println("extra fields   :", log.GetExtraFieldsList())

	log.Flush()
}

func printSeparator(title string) {
	fmt.Printf("\n=== %s ===\n\n", title)
}
