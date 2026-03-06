package redisdb

import (
	"context"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// NewRedisClient
// ---------------------------------------------------------------------------

func TestNewRedisClient(t *testing.T) {
	rc := NewRedisClient("test-cache", Config{
		Addrs: []string{"localhost:6379"},
	})
	if rc.GetClientName() != "test-cache" {
		t.Fatalf("expected test-cache, got %q", rc.GetClientName())
	}
	cfg := rc.GetConfig()
	if len(cfg.Addrs) != 1 || cfg.Addrs[0] != "localhost:6379" {
		t.Fatalf("expected config addr, got %v", cfg.Addrs)
	}
	if rc.GetClient() != nil {
		t.Fatal("client should be nil before Connect")
	}
}

// ---------------------------------------------------------------------------
// BlockingConfig
// ---------------------------------------------------------------------------

func TestBlockingConfig(t *testing.T) {
	base := Config{
		Addrs:      []string{"localhost:6379"},
		PoolSize:   20,
		MaxRetries: 5,
	}
	bc := BlockingConfig(base)

	if bc.ReadTimeout != -1 {
		t.Fatalf("expected ReadTimeout=-1, got %v", bc.ReadTimeout)
	}
	if bc.MaxRetries != -1 {
		t.Fatalf("expected MaxRetries=-1, got %v", bc.MaxRetries)
	}
	// Original values should be preserved
	if bc.PoolSize != 20 {
		t.Fatalf("expected PoolSize=20, got %d", bc.PoolSize)
	}
	// Base should not be modified (value type — copy semantics)
	if base.MaxRetries != 5 {
		t.Fatalf("base was mutated: MaxRetries=%d", base.MaxRetries)
	}
}

// ---------------------------------------------------------------------------
// applyDefaults
// ---------------------------------------------------------------------------

func TestApplyDefaults_ZeroConfig(t *testing.T) {
	c := applyDefaults(Config{})
	if c.DialTimeout != 5*time.Second {
		t.Fatalf("DialTimeout=%v, want 5s", c.DialTimeout)
	}
	if c.ReadTimeout != 3*time.Second {
		t.Fatalf("ReadTimeout=%v, want 3s", c.ReadTimeout)
	}
	if c.WriteTimeout != 3*time.Second {
		t.Fatalf("WriteTimeout=%v, want 3s", c.WriteTimeout)
	}
	if c.PoolSize != 10 {
		t.Fatalf("PoolSize=%d, want 10", c.PoolSize)
	}
	if c.MinIdleConns != 2 {
		t.Fatalf("MinIdleConns=%d, want 2", c.MinIdleConns)
	}
	if c.MaxIdleConns != 5 {
		t.Fatalf("MaxIdleConns=%d, want 5", c.MaxIdleConns)
	}
	if c.ConnMaxIdleTime != 5*time.Minute {
		t.Fatalf("ConnMaxIdleTime=%v, want 5m", c.ConnMaxIdleTime)
	}
	if c.ConnMaxLifetime != 30*time.Minute {
		t.Fatalf("ConnMaxLifetime=%v, want 30m", c.ConnMaxLifetime)
	}
	if c.MaxRetries != 3 {
		t.Fatalf("MaxRetries=%d, want 3", c.MaxRetries)
	}
	if c.MinRetryBackoff != 8*time.Millisecond {
		t.Fatalf("MinRetryBackoff=%v, want 8ms", c.MinRetryBackoff)
	}
	if c.MaxRetryBackoff != 512*time.Millisecond {
		t.Fatalf("MaxRetryBackoff=%v, want 512ms", c.MaxRetryBackoff)
	}
}

func TestApplyDefaults_PreservesExplicitValues(t *testing.T) {
	c := applyDefaults(Config{
		DialTimeout: 10 * time.Second,
		PoolSize:    50,
		MaxRetries:  7,
	})
	if c.DialTimeout != 10*time.Second {
		t.Fatalf("expected 10s, got %v", c.DialTimeout)
	}
	if c.PoolSize != 50 {
		t.Fatalf("expected 50, got %d", c.PoolSize)
	}
	if c.MaxRetries != 7 {
		t.Fatalf("expected 7, got %d", c.MaxRetries)
	}
}

func TestApplyDefaults_PreservesNegativeOne(t *testing.T) {
	c := applyDefaults(Config{
		ReadTimeout: -1,
		MaxRetries:  -1,
	})
	if c.ReadTimeout != -1 {
		t.Fatalf("expected -1, got %v", c.ReadTimeout)
	}
	if c.MaxRetries != -1 {
		t.Fatalf("expected -1, got %d", c.MaxRetries)
	}
}

// ---------------------------------------------------------------------------
// Getters / Setters
// ---------------------------------------------------------------------------

func TestClientName_GetSet(t *testing.T) {
	rc := NewRedisClient("original", Config{})
	rc.SetClientName("renamed")
	if rc.GetClientName() != "renamed" {
		t.Fatalf("expected renamed, got %q", rc.GetClientName())
	}
}

func TestConfig_GetSet(t *testing.T) {
	rc := NewRedisClient("test", Config{Addrs: []string{"a:1"}})
	rc.SetConfig(Config{Addrs: []string{"b:2", "c:3"}})
	cfg := rc.GetConfig()
	if len(cfg.Addrs) != 2 || cfg.Addrs[0] != "b:2" {
		t.Fatalf("expected updated config, got %v", cfg.Addrs)
	}
}

func TestClient_GetSet(t *testing.T) {
	rc := NewRedisClient("test", Config{})
	if rc.GetClient() != nil {
		t.Fatal("expected nil before set")
	}
	// We won't create a real redis client, just verify nil handling.
	rc.SetClient(nil)
	if rc.GetClient() != nil {
		t.Fatal("expected nil")
	}
}

// ---------------------------------------------------------------------------
// Connect validation (no real Redis)
// ---------------------------------------------------------------------------

func TestConnect_EmptyAddrs(t *testing.T) {
	rc := NewRedisClient("test", Config{})
	err := rc.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error for empty addrs")
	}
	if err.Error() != "redisdb: at least one address is required in Config.Addrs" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnect_InvalidPoolHierarchy_MinGtMax(t *testing.T) {
	rc := NewRedisClient("test", Config{
		Addrs:        []string{"localhost:6379"},
		MinIdleConns: 10,
		MaxIdleConns: 5,
		PoolSize:     20,
	})
	err := rc.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error for MinIdleConns > MaxIdleConns")
	}
}

func TestConnect_InvalidPoolHierarchy_MaxGtPool(t *testing.T) {
	rc := NewRedisClient("test", Config{
		Addrs:        []string{"localhost:6379"},
		MinIdleConns: 2,
		MaxIdleConns: 50,
		PoolSize:     10,
	})
	err := rc.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error for MaxIdleConns > PoolSize")
	}
}

func TestConnect_NilContext(t *testing.T) {
	// Should use context.Background() internally and fail on PING, not nil deref
	rc := NewRedisClient("test", Config{
		Addrs: []string{"localhost:19999"}, // non-existent
	})
	err := rc.Connect(nil)
	if err == nil {
		t.Fatal("expected connection error, not nil")
	}
}

// ---------------------------------------------------------------------------
// Close (no real connection)
// ---------------------------------------------------------------------------

func TestClose_NilClient(t *testing.T) {
	rc := NewRedisClient("test", Config{})
	// Close on nil client should return nil
	if err := rc.Close(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Global convenience functions
// ---------------------------------------------------------------------------

func TestGlobal_DefaultName(t *testing.T) {
	if GetClientName() != "globalRedisClient" {
		t.Fatalf("expected globalRedisClient, got %q", GetClientName())
	}
}

func TestGlobal_SetGetConfig(t *testing.T) {
	SetConfig(Config{Addrs: []string{"redis:6379"}, PoolSize: 20})
	cfg := GetConfig()
	if cfg.PoolSize != 20 {
		t.Fatalf("expected 20, got %d", cfg.PoolSize)
	}
}

func TestGlobal_SetGetClientName(t *testing.T) {
	SetClientName("my-global")
	if GetClientName() != "my-global" {
		t.Fatalf("expected my-global, got %q", GetClientName())
	}
	SetClientName("globalRedisClient") // restore
}

func TestGlobal_SetGetClient(t *testing.T) {
	if GetClient() != nil {
		t.Fatal("expected nil default client")
	}
	SetClient(nil) // no-op, shouldn't crash
}

func TestGlobal_ConnectEmptyAddrs(t *testing.T) {
	SetConfig(Config{}) // empty
	err := GlobalConnect(context.Background())
	if err == nil {
		t.Fatal("expected error for empty addrs")
	}
}

func TestGlobal_CloseNil(t *testing.T) {
	if err := GlobalClose(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
