package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gracefulshutdown "github.com/cyrus-wg/gobox/pkg/graceful_shutdown"
	"github.com/cyrus-wg/gobox/pkg/logger"
)

// ──────────────────────────────────────────────
// This example demonstrates EVERY code path in
// the graceful_shutdown package in a single run.
//
// Covered scenarios:
//
//  Servers:
//   • Normal server shutdown (public-api, health)
//   • Nil server entry → skipped with WARN
//   • Already-closed server → logs http.ErrServerClosed
//   • Server with empty Name → auto-generated "server-4"
//   • Server with zero Timeout → defaults to 15 s
//
//  Cleanup hooks:
//   • Successful hook (close-db)
//   • Hook that returns an error (flush-cache)
//   • Hook that panics → recovered, next hooks run
//   • Nil Fn → skipped with WARN
//   • Hook that exceeds its timeout → context.DeadlineExceeded
//   • Hook with empty Name → auto-generated "cleanup-5"
//   • Hook with zero Timeout → defaults to 15 s (flush-logger)
//
// Run:
//
//wsl\$/go run ./example/graceful_shutdown/
//
// Then press Ctrl+C (SIGINT) to trigger the
// graceful shutdown and inspect the structured
// JSON log output.
// ──────────────────────────────────────────────

func main() {
	logger.InitGlobalLogger(logger.LoggerConfig{
		DebugLogLevel:   true,
		RequestIDPrefix: "gs-",
		FixedKeyValues: map[string]any{
			"app": "graceful-shutdown-example",
		},
	})

	ctx := context.Background()

	// ── Start HTTP servers ──────────────────────────────────────────────

	// Server 1: Public API on :8080
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello from the public API!")
	})
	apiServer := &http.Server{Addr: ":8080", Handler: apiMux}

	go func() {
		logger.Infow(ctx, "Starting public API server", "addr", apiServer.Addr)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorw(ctx, "Public API server error", "error", err)
		}
	}()

	// Server 2: Internal health/metrics on :8081
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	healthServer := &http.Server{Addr: ":8081", Handler: healthMux}

	go func() {
		logger.Infow(ctx, "Starting health server", "addr", healthServer.Addr)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorw(ctx, "Health server error", "error", err)
		}
	}()

	// Server 3: Already-closed server — Shutdown() on this server will
	// report http.ErrServerClosed.
	closedServer := &http.Server{Addr: ":8082"}
	closedServer.Close() // close immediately so Shutdown() sees an error

	// Give servers a moment to bind their ports.
	time.Sleep(100 * time.Millisecond)

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Servers running on :8080 (API) and :8081 (health)                ║")
	fmt.Println("║  Press Ctrl+C to trigger graceful shutdown.                       ║")
	fmt.Println("║                                                                   ║")
	fmt.Println("║  Watch the logs — every Shutdown code path will fire:              ║")
	fmt.Println("║    servers : normal, nil, already-closed, empty-name, zero-timeout ║")
	fmt.Println("║    cleanup : ok, error, panic, nil, timeout, empty-name, default  ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// ── Configure and block on Shutdown ─────────────────────────────────
	//
	// What this demonstrates:
	//
	//  Servers (stopped sequentially in listed order):
	//   0. "public-api"        — normal shutdown with 10 s timeout.
	//   1. "nil-server"        — nil Server → skipped with WARN.
	//   2. "already-closed"    — server already closed → logs error.
	//   3. "health"            — normal shutdown with 5 s timeout.
	//   4. (empty Name)        — auto-generated name "server-4".
	//                            Zero Timeout → defaults to 15 s.
	//
	//  Cleanup hooks (run sequentially after all servers stop):
	//   0. "close-db"          — success: simulates closing a connection pool.
	//   1. "flush-cache"       — error: returns a partial-failure error.
	//   2. "broken-hook"       — panic: recovered so the next hook runs.
	//   3. "nil-cleanup"       — nil Fn → skipped with WARN.
	//   4. "slow-hook"         — exceeds its 1 s timeout → context deadline.
	//   5. (empty Name)        — auto-generated name "cleanup-5".
	//                            Zero Timeout → defaults to 15 s.
	//   6. "flush-logger"      — always runs last; flushes the zap logger.

	gracefulshutdown.Shutdown(ctx, gracefulshutdown.ShutdownConfig{
		HardTimeout: 30 * time.Second,

		Servers: []gracefulshutdown.ServerEntry{
			// 0: Normal server shutdown.
			{
				Name:    "public-api",
				Server:  apiServer,
				Timeout: 10 * time.Second,
			},
			// 1: Nil server entry — Shutdown logs WARN and skips.
			{
				Name:   "nil-server",
				Server: nil,
			},
			// 2: Already-closed server — Shutdown logs error.
			{
				Name:    "already-closed",
				Server:  closedServer,
				Timeout: 3 * time.Second,
			},
			// 3: Normal server shutdown (different timeout).
			{
				Name:    "health",
				Server:  healthServer,
				Timeout: 5 * time.Second,
			},
			// 4: Empty Name → auto-generated "server-4".
			//
			//	Zero Timeout → defaults to 15 s.
			{
				Server: apiServer, // already stopped; tests double-shutdown
			},
		},

		OnShutdown: []gracefulshutdown.CleanupEntry{
			// 0: Success path.
			{
				Name:    "close-db",
				Timeout: 5 * time.Second,
				Fn: func(ctx context.Context) error {
					logger.Infow(ctx, "Closing database connections …")
					time.Sleep(200 * time.Millisecond)
					logger.Infow(ctx, "Database connections closed")
					return nil
				},
			},
			// 1: Returns an error.
			{
				Name:    "flush-cache",
				Timeout: 3 * time.Second,
				Fn: func(ctx context.Context) error {
					logger.Infow(ctx, "Flushing cache …")
					time.Sleep(100 * time.Millisecond)
					return fmt.Errorf("cache flush: 2 keys failed to evict")
				},
			},
			// 2: Panics — recovered so subsequent hooks still execute.
			{
				Name:    "broken-hook",
				Timeout: 2 * time.Second,
				Fn: func(ctx context.Context) error {
					panic("unexpected nil pointer in telemetry exporter")
				},
			},
			// 3: Nil Fn — Shutdown logs WARN and skips.
			{
				Name: "nil-cleanup",
				Fn:   nil,
			},
			// 4: Exceeds its per-item timeout (1 s) — logs context deadline error.
			{
				Name:    "slow-hook",
				Timeout: 1 * time.Second,
				Fn: func(ctx context.Context) error {
					logger.Infow(ctx, "Slow hook started, will exceed 1 s timeout …")
					select {
					case <-time.After(5 * time.Second):
						return nil // never reached
					case <-ctx.Done():
						return ctx.Err() // context.DeadlineExceeded
					}
				},
			},
			// 5: Empty Name → auto-generated "cleanup-5".
			//
			//	Zero Timeout → defaults to 15 s.
			{
				Fn: func(ctx context.Context) error {
					logger.Infow(ctx, "Unnamed cleanup hook running (auto-generated name)")
					return nil
				},
			},
			// 6: Final hook — flush the logger so all output lands on disk.
			{
				Name:    "flush-logger",
				Timeout: 2 * time.Second,
				Fn: func(ctx context.Context) error {
					logger.Flush()
					return nil
				},
			},
		},
	})

	fmt.Println()
	fmt.Println("Process exited cleanly.")
}
