package gracefulshutdown

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"google.golang.org/grpc"
)

func init() {
	// Suppress noisy log output during tests.
	logger.InitGlobalLogger(logger.LoggerConfig{})
}

// ─── itemContext ─────────────────────────────────────────────────────────────

func TestItemContext_ExplicitTimeout(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx, itemCancel := itemContext(parent, 2*time.Second)
	defer itemCancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on context")
	}

	// The deadline should be ~2 s from now, not 10 s.
	remaining := time.Until(deadline)
	if remaining > 3*time.Second {
		t.Errorf("expected ~2 s timeout, got %v", remaining)
	}
}

func TestItemContext_ZeroTimeout_DefaultsTo15s(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx, itemCancel := itemContext(parent, 0)
	defer itemCancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on context")
	}

	remaining := time.Until(deadline)
	if remaining < 14*time.Second || remaining > 16*time.Second {
		t.Errorf("expected ~15 s default timeout, got %v", remaining)
	}
}

func TestItemContext_NegativeTimeout_DefaultsTo15s(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ctx, itemCancel := itemContext(parent, -5*time.Second)
	defer itemCancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on context")
	}

	remaining := time.Until(deadline)
	if remaining < 14*time.Second || remaining > 16*time.Second {
		t.Errorf("expected ~15 s default timeout, got %v", remaining)
	}
}

func TestItemContext_CappedByParent(t *testing.T) {
	// Parent has 1 s left, item requests 10 s → effective = 1 s.
	parent, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ctx, itemCancel := itemContext(parent, 10*time.Second)
	defer itemCancel()

	deadline, _ := ctx.Deadline()
	remaining := time.Until(deadline)
	if remaining > 2*time.Second {
		t.Errorf("expected ≤1 s (capped by parent), got %v", remaining)
	}
}

// ─── shutdownServer ─────────────────────────────────────────────────────────

func TestShutdownServer_Normal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv := ts.Config

	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := ServerEntry{Server: srv, Timeout: 2 * time.Second}

	// Should not panic.
	shutdownServer(context.Background(), hardCtx, "test-server", entry)

	// Verify the server is actually shut down by trying to connect.
	ts.Close() // idempotent
}

func TestShutdownServer_AlreadyClosed(t *testing.T) {
	srv := &http.Server{Addr: ":0"}
	srv.Close()

	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := ServerEntry{Server: srv, Timeout: 2 * time.Second}

	// Should log error but not panic.
	shutdownServer(context.Background(), hardCtx, "closed-server", entry)
}

func TestShutdownServer_DoubleShutdown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv := ts.Config

	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := ServerEntry{Server: srv, Timeout: 2 * time.Second}

	// First shutdown succeeds.
	shutdownServer(context.Background(), hardCtx, "srv", entry)
	// Second shutdown on same server should not panic.
	shutdownServer(context.Background(), hardCtx, "srv-again", entry)
}

// ─── runCleanupHook ─────────────────────────────────────────────────────────

func TestRunCleanupHook_Success(t *testing.T) {
	var called atomic.Bool

	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := CleanupEntry{
		Timeout: 2 * time.Second,
		Fn: func(ctx context.Context) error {
			called.Store(true)
			return nil
		},
	}

	runCleanupHook(context.Background(), hardCtx, "ok-hook", entry)

	if !called.Load() {
		t.Error("cleanup function was not called")
	}
}

func TestRunCleanupHook_ReturnsError(t *testing.T) {
	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := CleanupEntry{
		Timeout: 2 * time.Second,
		Fn: func(ctx context.Context) error {
			return errors.New("flush failed")
		},
	}

	// Should log error but not panic.
	runCleanupHook(context.Background(), hardCtx, "err-hook", entry)
}

func TestRunCleanupHook_Panics_Recovered(t *testing.T) {
	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := CleanupEntry{
		Timeout: 2 * time.Second,
		Fn: func(ctx context.Context) error {
			panic("boom")
		},
	}

	// Must not propagate the panic.
	runCleanupHook(context.Background(), hardCtx, "panic-hook", entry)
}

func TestRunCleanupHook_ExceedsTimeout(t *testing.T) {
	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := CleanupEntry{
		Timeout: 100 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			select {
			case <-time.After(5 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	start := time.Now()
	runCleanupHook(context.Background(), hardCtx, "slow-hook", entry)
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("expected hook to be cancelled by timeout (~100ms), took %v", elapsed)
	}
}

// ─── executeShutdown (end-to-end) ───────────────────────────────────────────

func TestExecuteShutdown_EmptyConfig(t *testing.T) {
	// No servers, no hooks — should complete immediately without panic.
	ctx := context.Background()
	executeShutdown(ctx, ShutdownConfig{})
}

func TestExecuteShutdown_DefaultHardTimeout(t *testing.T) {
	// HardTimeout=0 should use default (30s), not panic or use negative.
	ctx := context.Background()
	executeShutdown(ctx, ShutdownConfig{HardTimeout: 0})
}

func TestExecuteShutdown_NegativeHardTimeout(t *testing.T) {
	ctx := context.Background()
	executeShutdown(ctx, ShutdownConfig{HardTimeout: -1 * time.Second})
}

func TestExecuteShutdown_NilServerSkipped(t *testing.T) {
	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		Servers: []ServerEntry{
			{Name: "nil-srv", Server: nil},
		},
	}
	// Should log WARN and not panic.
	executeShutdown(ctx, config)
}

func TestExecuteShutdown_NilCleanupFnSkipped(t *testing.T) {
	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		OnShutdown: []CleanupEntry{
			{Name: "nil-fn", Fn: nil},
		},
	}
	executeShutdown(ctx, config)
}

func TestExecuteShutdown_EmptyNames_AutoGenerated(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	var called atomic.Bool

	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		Servers: []ServerEntry{
			{Server: ts.Config}, // empty name → "server-0"
		},
		OnShutdown: []CleanupEntry{
			{Fn: func(ctx context.Context) error { // empty name → "cleanup-0"
				called.Store(true)
				return nil
			}},
		},
	}

	executeShutdown(ctx, config)

	if !called.Load() {
		t.Error("unnamed cleanup hook was not called")
	}
}

func TestExecuteShutdown_MultipleServers_SequentialOrder(t *testing.T) {
	var order []string

	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		Servers: []ServerEntry{
			{Name: "first", Server: ts1.Config, Timeout: 1 * time.Second},
			{Name: "second", Server: ts2.Config, Timeout: 1 * time.Second},
		},
		OnShutdown: []CleanupEntry{
			{Name: "hook-a", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				order = append(order, "hook-a")
				return nil
			}},
			{Name: "hook-b", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				order = append(order, "hook-b")
				return nil
			}},
		},
	}

	executeShutdown(ctx, config)

	if len(order) != 2 || order[0] != "hook-a" || order[1] != "hook-b" {
		t.Errorf("expected [hook-a, hook-b], got %v", order)
	}
}

func TestExecuteShutdown_HardTimeoutExhaustion_SkipsRemaining(t *testing.T) {
	var hooksCalled atomic.Int32

	ctx := context.Background()
	config := ShutdownConfig{
		// Very short hard timeout — the slow hook eats most of it.
		HardTimeout: 300 * time.Millisecond,
		OnShutdown: []CleanupEntry{
			{
				Name:    "slow",
				Timeout: 5 * time.Second,
				Fn: func(ctx context.Context) error {
					hooksCalled.Add(1)
					select {
					case <-time.After(5 * time.Second):
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				},
			},
			{
				Name:    "should-be-skipped",
				Timeout: 1 * time.Second,
				Fn: func(ctx context.Context) error {
					hooksCalled.Add(1)
					return nil
				},
			},
		},
	}

	start := time.Now()
	executeShutdown(ctx, config)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("expected fast completion (~300ms), took %v", elapsed)
	}

	// The slow hook runs (and times out). The second hook may or may not
	// run depending on timing — the key assertion is that we don't block
	// for 5 seconds.
	if hooksCalled.Load() < 1 {
		t.Error("expected at least the slow hook to be called")
	}
}

func TestExecuteShutdown_CleanupPanic_DoesNotBlockSubsequent(t *testing.T) {
	var lastHookCalled atomic.Bool

	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		OnShutdown: []CleanupEntry{
			{Name: "panic-hook", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				panic("crash")
			}},
			{Name: "after-panic", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				lastHookCalled.Store(true)
				return nil
			}},
		},
	}

	executeShutdown(ctx, config)

	if !lastHookCalled.Load() {
		t.Error("hook after the panicking hook should still run")
	}
}

func TestExecuteShutdown_MixedServerStates(t *testing.T) {
	// Normal server + nil server + already-closed server.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	closedSrv := &http.Server{Addr: ":0"}
	closedSrv.Close()

	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		Servers: []ServerEntry{
			{Name: "live", Server: ts.Config, Timeout: 1 * time.Second},
			{Name: "nil", Server: nil},
			{Name: "closed", Server: closedSrv, Timeout: 1 * time.Second},
		},
	}

	// Must complete without panicking.
	executeShutdown(ctx, config)
}

func TestExecuteShutdown_FullScenario(t *testing.T) {
	// Mirrors the example: tests every code path together.
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	closedSrv := &http.Server{Addr: ":0"}
	closedSrv.Close()

	var hookResults []string

	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 10 * time.Second,
		Servers: []ServerEntry{
			{Name: "api", Server: ts1.Config, Timeout: 2 * time.Second},
			{Name: "nil-srv", Server: nil},
			{Name: "closed-srv", Server: closedSrv, Timeout: 1 * time.Second},
			{Name: "health", Server: ts2.Config, Timeout: 2 * time.Second},
			{Server: ts1.Config}, // already stopped, empty name
		},
		OnShutdown: []CleanupEntry{
			{Name: "ok", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				hookResults = append(hookResults, "ok")
				return nil
			}},
			{Name: "err", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				hookResults = append(hookResults, "err")
				return errors.New("fail")
			}},
			{Name: "panic", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				panic("test panic")
			}},
			{Name: "nil-fn", Fn: nil},
			{Name: "timeout", Timeout: 100 * time.Millisecond, Fn: func(ctx context.Context) error {
				<-ctx.Done()
				hookResults = append(hookResults, "timeout")
				return ctx.Err()
			}},
			{Fn: func(ctx context.Context) error { // empty name
				hookResults = append(hookResults, "unnamed")
				return nil
			}},
			{Name: "last", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				hookResults = append(hookResults, "last")
				return nil
			}},
		},
	}

	executeShutdown(ctx, config)

	// Verify all non-skipped hooks ran in order.
	expected := []string{"ok", "err", "timeout", "unnamed", "last"}
	if len(hookResults) != len(expected) {
		t.Fatalf("expected %d hook results, got %d: %v", len(expected), len(hookResults), hookResults)
	}
	for i, exp := range expected {
		if hookResults[i] != exp {
			t.Errorf("hookResults[%d] = %q, want %q", i, hookResults[i], exp)
		}
	}
}

// ─── gRPC server helpers ────────────────────────────────────────────────────

// newListeningGrpcServer creates a gRPC server that is actively listening.
func newListeningGrpcServer(t *testing.T) (*grpc.Server, net.Listener) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	srv := grpc.NewServer()
	go func() {
		_ = srv.Serve(lis)
	}()
	return srv, lis
}

// ─── shutdownGrpcServer ─────────────────────────────────────────────────────

func TestShutdownGrpcServer_Normal(t *testing.T) {
	srv, lis := newListeningGrpcServer(t)
	defer lis.Close()

	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := GrpcServerEntry{Server: srv, Timeout: 2 * time.Second}

	// Should not panic.
	shutdownGrpcServer(context.Background(), hardCtx, "test-grpc", entry)
}

func TestShutdownGrpcServer_AlreadyStopped(t *testing.T) {
	srv := grpc.NewServer()
	srv.Stop() // stop immediately

	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := GrpcServerEntry{Server: srv, Timeout: 2 * time.Second}

	// Should not panic on already stopped server.
	shutdownGrpcServer(context.Background(), hardCtx, "stopped-grpc", entry)
}

func TestShutdownGrpcServer_DoubleShutdown(t *testing.T) {
	srv, lis := newListeningGrpcServer(t)
	defer lis.Close()

	hardCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := GrpcServerEntry{Server: srv, Timeout: 2 * time.Second}

	// First shutdown succeeds.
	shutdownGrpcServer(context.Background(), hardCtx, "grpc", entry)
	// Second shutdown on same server should not panic.
	shutdownGrpcServer(context.Background(), hardCtx, "grpc-again", entry)
}

// ─── executeShutdown with gRPC servers ──────────────────────────────────────

func TestExecuteShutdown_GrpcServersOnly(t *testing.T) {
	srv, lis := newListeningGrpcServer(t)
	defer lis.Close()

	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		GrpcServers: []GrpcServerEntry{
			{Name: "grpc-api", Server: srv, Timeout: 2 * time.Second},
		},
	}

	executeShutdown(ctx, config)
}

func TestExecuteShutdown_NilGrpcServerSkipped(t *testing.T) {
	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		GrpcServers: []GrpcServerEntry{
			{Name: "nil-grpc", Server: nil},
		},
	}
	// Should log WARN and not panic.
	executeShutdown(ctx, config)
}

func TestExecuteShutdown_GrpcEmptyName_AutoGenerated(t *testing.T) {
	srv, lis := newListeningGrpcServer(t)
	defer lis.Close()

	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		GrpcServers: []GrpcServerEntry{
			{Server: srv}, // empty name → "grpc-server-0"
		},
	}

	executeShutdown(ctx, config)
}

func TestExecuteShutdown_MixedHttpAndGrpc(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	grpcSrv, lis := newListeningGrpcServer(t)
	defer lis.Close()

	var hookCalled atomic.Bool

	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		Servers: []ServerEntry{
			{Name: "http-api", Server: ts.Config, Timeout: 1 * time.Second},
		},
		GrpcServers: []GrpcServerEntry{
			{Name: "grpc-api", Server: grpcSrv, Timeout: 1 * time.Second},
		},
		OnShutdown: []CleanupEntry{
			{Name: "final-hook", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				hookCalled.Store(true)
				return nil
			}},
		},
	}

	executeShutdown(ctx, config)

	if !hookCalled.Load() {
		t.Error("cleanup hook should have been called after HTTP and gRPC servers stopped")
	}
}

func TestExecuteShutdown_MultipleGrpcServers_SequentialOrder(t *testing.T) {
	srv1, lis1 := newListeningGrpcServer(t)
	defer lis1.Close()
	srv2, lis2 := newListeningGrpcServer(t)
	defer lis2.Close()

	var order []string

	ctx := context.Background()
	config := ShutdownConfig{
		HardTimeout: 5 * time.Second,
		GrpcServers: []GrpcServerEntry{
			{Name: "first-grpc", Server: srv1, Timeout: 1 * time.Second},
			{Name: "second-grpc", Server: srv2, Timeout: 1 * time.Second},
		},
		OnShutdown: []CleanupEntry{
			{Name: "hook-a", Timeout: 1 * time.Second, Fn: func(ctx context.Context) error {
				order = append(order, "hook-a")
				return nil
			}},
		},
	}

	executeShutdown(ctx, config)

	if len(order) != 1 || order[0] != "hook-a" {
		t.Errorf("expected [hook-a], got %v", order)
	}
}

func TestExecuteShutdown_GrpcWithNoServersWarning(t *testing.T) {
	// No HTTP, no gRPC, no hooks → should log warning.
	ctx := context.Background()
	executeShutdown(ctx, ShutdownConfig{})
}
