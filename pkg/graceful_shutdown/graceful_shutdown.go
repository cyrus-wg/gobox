package gracefulshutdown

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
)

const (
	// defaultHardTimeout is applied when ShutdownConfig.HardTimeout is zero.
	defaultHardTimeout = 30 * time.Second

	// defaultItemTimeout is applied when a ServerEntry or CleanupEntry
	// has no explicit Timeout.
	defaultItemTimeout = 15 * time.Second
)

// CleanupFunc is executed during the cleanup phase of a graceful shutdown.
// It receives a context bounded by min(its own Timeout, remaining HardTimeout).
type CleanupFunc func(ctx context.Context) error

// ServerEntry pairs an HTTP server with per-server shutdown settings.
type ServerEntry struct {
	// Server is the *http.Server to shut down.
	Server *http.Server

	// Name is a human-readable label used in log messages.
	// If empty, a default "server-<index>" is generated.
	Name string

	// Timeout is the maximum duration given to this server's Shutdown call.
	// The effective timeout is min(Timeout, remaining HardTimeout).
	// Zero or negative values default to 15 s.
	Timeout time.Duration
}

// CleanupEntry pairs a cleanup function with per-hook settings.
type CleanupEntry struct {
	// Name is a human-readable label used in log messages.
	// If empty, a default "cleanup-<index>" is generated.
	Name string

	// Fn is the cleanup function to execute.
	Fn CleanupFunc

	// Timeout is the maximum duration given to this hook.
	// The effective timeout is min(Timeout, remaining HardTimeout).
	// Zero or negative values default to 15 s.
	Timeout time.Duration
}

// ShutdownConfig configures the graceful shutdown behaviour.
type ShutdownConfig struct {
	// HardTimeout is the absolute upper bound for the entire shutdown
	// process (all server shutdowns + all cleanup hooks combined).
	// Once exceeded, remaining operations are skipped.
	// Zero or negative values default to 30 s.
	HardTimeout time.Duration

	// Servers lists the HTTP servers to shut down. Servers are stopped
	// sequentially in the order provided so that callers can control
	// dependency ordering — e.g. stop the public API first, then the
	// internal health/metrics server last.
	// Each server may specify its own Timeout.
	Servers []ServerEntry

	// OnShutdown holds cleanup functions that run sequentially (in order)
	// after every server has been stopped. Common uses include closing
	// database connections, flushing telemetry, and releasing external
	// resources. Each hook may specify its own Timeout.
	OnShutdown []CleanupEntry
}

// Shutdown blocks until an OS interrupt or SIGTERM is received, then
// performs a two-phase graceful shutdown:
//
//  1. All configured HTTP servers are shut down sequentially so callers
//     can control the teardown order. Each server gets its own timeout
//     capped by the global hard timeout.
//  2. OnShutdown hooks execute sequentially with individual timeouts,
//     also capped by the remaining hard timeout.
//
// If a second signal arrives while shutdown is in progress the process
// exits immediately with code 1.
//
// Panics inside server shutdown or cleanup hooks are recovered and logged
// so that remaining operations still execute.
func Shutdown(ctx context.Context, config ShutdownConfig) {
	if len(config.Servers) == 0 && len(config.OnShutdown) == 0 {
		logger.Warnw(ctx, "Shutdown called with no servers and no cleanup hooks")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Infow(ctx, "Received shutdown signal", "signal", sig)

	// A second signal forces an immediate exit.
	go func() {
		sig := <-quit
		logger.Warnw(ctx, "Received second signal, forcing immediate exit", "signal", sig)
		os.Exit(1)
	}()

	defer signal.Stop(quit)

	executeShutdown(ctx, config)
}

// executeShutdown is the core shutdown logic, separated from signal handling
// so it can be tested without OS signals.
func executeShutdown(ctx context.Context, config ShutdownConfig) {
	hardTimeout := config.HardTimeout
	if hardTimeout <= 0 {
		hardTimeout = defaultHardTimeout
	}

	hardCtx, hardCancel := context.WithTimeout(ctx, hardTimeout)
	defer hardCancel()

	shutdownStart := time.Now()

	// ── Phase 1: stop HTTP servers sequentially ─────────────────────────
	logger.Infow(ctx, "Shutting down HTTP servers",
		"serverCount", len(config.Servers),
		"hardTimeout", hardTimeout.String(),
	)

	for i, entry := range config.Servers {
		if hardCtx.Err() != nil {
			logger.Warnw(ctx, "Hard timeout reached, skipping remaining servers",
				"skipped", len(config.Servers)-i,
			)
			break
		}

		name := entry.Name
		if name == "" {
			name = fmt.Sprintf("server-%d", i)
		}

		if entry.Server == nil {
			logger.Warnw(ctx, "Skipping nil server entry", "name", name)
			continue
		}

		shutdownServer(ctx, hardCtx, name, entry)
	}

	logger.Infow(ctx, "All HTTP servers stopped",
		"duration", time.Since(shutdownStart).String(),
	)

	// ── Phase 2: run cleanup hooks sequentially ─────────────────────────
	if len(config.OnShutdown) > 0 {
		logger.Infow(ctx, "Running cleanup hooks", "hookCount", len(config.OnShutdown))

		for i, entry := range config.OnShutdown {
			if hardCtx.Err() != nil {
				logger.Warnw(ctx, "Hard timeout reached, skipping remaining cleanup hooks",
					"skipped", len(config.OnShutdown)-i,
				)
				break
			}

			name := entry.Name
			if name == "" {
				name = fmt.Sprintf("cleanup-%d", i)
			}

			if entry.Fn == nil {
				logger.Warnw(ctx, "Skipping nil cleanup function", "name", name)
				continue
			}

			runCleanupHook(ctx, hardCtx, name, entry)
		}
	}

	logger.Infow(ctx, "Graceful shutdown completed",
		"totalDuration", time.Since(shutdownStart).String(),
	)
}

// shutdownServer stops a single HTTP server with panic recovery.
func shutdownServer(ctx context.Context, hardCtx context.Context, name string, entry ServerEntry) {
	itemCtx, itemCancel := itemContext(hardCtx, entry.Timeout)
	defer itemCancel()

	start := time.Now()
	logger.Infow(ctx, "Stopping server", "name", name)

	defer func() {
		if r := recover(); r != nil {
			logger.Errorw(ctx, "Panic during server shutdown",
				"name", name,
				"panic", fmt.Sprintf("%v", r),
				"duration", time.Since(start).String(),
			)
		}
	}()

	if err := entry.Server.Shutdown(itemCtx); err != nil {
		logger.Errorw(ctx, "Server shutdown error",
			"name", name,
			"error", err,
			"duration", time.Since(start).String(),
		)
	} else {
		logger.Infow(ctx, "Server stopped",
			"name", name,
			"duration", time.Since(start).String(),
		)
	}
}

// runCleanupHook executes a single cleanup function with panic recovery.
func runCleanupHook(ctx context.Context, hardCtx context.Context, name string, entry CleanupEntry) {
	itemCtx, itemCancel := itemContext(hardCtx, entry.Timeout)
	defer itemCancel()

	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			logger.Errorw(ctx, "Panic during cleanup hook",
				"name", name,
				"panic", fmt.Sprintf("%v", r),
				"duration", time.Since(start).String(),
			)
		}
	}()

	if err := entry.Fn(itemCtx); err != nil {
		logger.Errorw(ctx, "Cleanup hook error",
			"name", name,
			"error", err,
			"duration", time.Since(start).String(),
		)
	} else {
		logger.Infow(ctx, "Cleanup hook completed",
			"name", name,
			"duration", time.Since(start).String(),
		)
	}
}

// itemContext returns a child context bounded by the given timeout.
// Because the parent (hardCtx) already carries the hard deadline,
// context.WithTimeout naturally yields min(timeout, remaining hard deadline).
func itemContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultItemTimeout
	}
	return context.WithTimeout(parent, timeout)
}
