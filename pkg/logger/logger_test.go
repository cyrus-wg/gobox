package logger

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewLogger / LoggerConfig
// ---------------------------------------------------------------------------

func TestNewLogger_DefaultConfig(t *testing.T) {
	l := NewLogger(LoggerConfig{})
	if l.IsDebugLogLevel() {
		t.Fatal("default should not be debug")
	}
	if l.GetRequestIDPrefix() != "" {
		t.Fatalf("expected empty prefix, got %q", l.GetRequestIDPrefix())
	}
	if l.GetFixedKeyValues() != nil {
		t.Fatal("expected nil fixed key values")
	}
}

func TestNewLogger_DebugLevel(t *testing.T) {
	l := NewLogger(LoggerConfig{DebugLogLevel: true})
	if !l.IsDebugLogLevel() {
		t.Fatal("expected debug log level")
	}
}

func TestNewLogger_WithPrefix(t *testing.T) {
	l := NewLogger(LoggerConfig{RequestIDPrefix: "SVC-"})
	if l.GetRequestIDPrefix() != "SVC-" {
		t.Fatalf("expected SVC-, got %q", l.GetRequestIDPrefix())
	}
}

func TestNewLogger_FixedKeyValues(t *testing.T) {
	fkv := map[string]any{"env": "test", "version": "1.0"}
	l := NewLogger(LoggerConfig{FixedKeyValues: fkv})
	got := l.GetFixedKeyValues()
	if got["env"] != "test" || got["version"] != "1.0" {
		t.Fatalf("expected fixed key values, got %v", got)
	}
}

func TestNewLogger_ExtraFields(t *testing.T) {
	l := NewLogger(LoggerConfig{ExtraFields: []string{"tenant_id", "user_id"}})
	fields := l.GetExtraFieldsList()
	if len(fields) != 2 || fields[0] != "tenant_id" {
		t.Fatalf("expected extra fields, got %v", fields)
	}
}

// ---------------------------------------------------------------------------
// GenerateRequestID
// ---------------------------------------------------------------------------

func TestGenerateRequestID_NoPrefix(t *testing.T) {
	l := NewLogger(LoggerConfig{})
	id := l.GenerateRequestID()
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
	// UUID is 36 chars (8-4-4-4-12)
	if len(id) != 36 {
		t.Fatalf("expected 36-char UUID, got %d: %q", len(id), id)
	}
}

func TestGenerateRequestID_WithPrefix(t *testing.T) {
	l := NewLogger(LoggerConfig{RequestIDPrefix: "API-"})
	id := l.GenerateRequestID()
	if !strings.HasPrefix(id, "API-") {
		t.Fatalf("expected API- prefix, got %q", id)
	}
	if len(id) != 40 { // 4 (prefix) + 36 (UUID)
		t.Fatalf("expected 40 chars, got %d: %q", len(id), id)
	}
}

func TestGenerateRequestID_Unique(t *testing.T) {
	l := NewLogger(LoggerConfig{})
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := l.GenerateRequestID()
		if ids[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

// ---------------------------------------------------------------------------
// SetRequestID / GetRequestID
// ---------------------------------------------------------------------------

func TestSetGetRequestID(t *testing.T) {
	l := NewLogger(LoggerConfig{})
	ctx := l.SetRequestID(context.Background(), "my-request-123")
	id, ok := l.GetRequestID(ctx)
	if !ok {
		t.Fatal("expected request ID in context")
	}
	if id != "my-request-123" {
		t.Fatalf("expected my-request-123, got %q", id)
	}
}

func TestGetRequestID_NotSet(t *testing.T) {
	l := NewLogger(LoggerConfig{})
	_, ok := l.GetRequestID(context.Background())
	if ok {
		t.Fatal("expected false when request ID not set")
	}
}

// ---------------------------------------------------------------------------
// GetExtraFields
// ---------------------------------------------------------------------------

func TestGetExtraFields_NoConfig(t *testing.T) {
	l := NewLogger(LoggerConfig{})
	_, ok := l.GetExtraFields(context.Background())
	if ok {
		t.Fatal("expected false with no extra fields configured")
	}
}

func TestGetExtraFields_WithValues(t *testing.T) {
	l := NewLogger(LoggerConfig{ExtraFields: []string{"tenant_id", "user_id"}})
	ctx := context.WithValue(context.Background(), "tenant_id", "t-123")
	ctx = context.WithValue(ctx, "user_id", "u-456")

	fields, ok := l.GetExtraFields(ctx)
	if !ok {
		t.Fatal("expected true")
	}
	if fields["tenant_id"] != "t-123" {
		t.Fatalf("expected t-123, got %v", fields["tenant_id"])
	}
	if fields["user_id"] != "u-456" {
		t.Fatalf("expected u-456, got %v", fields["user_id"])
	}
}

func TestGetExtraFields_PartialValues(t *testing.T) {
	l := NewLogger(LoggerConfig{ExtraFields: []string{"a", "b"}})
	ctx := context.WithValue(context.Background(), "a", "val")
	// "b" not set

	fields, ok := l.GetExtraFields(ctx)
	if !ok {
		t.Fatal("expected true (at least one field present is still a valid map)")
	}
	if fields["a"] != "val" {
		t.Fatalf("expected val, got %v", fields["a"])
	}
	if _, exists := fields["b"]; exists {
		t.Fatal("b should not be in fields")
	}
}

// ---------------------------------------------------------------------------
// combineAttributes
// ---------------------------------------------------------------------------

func TestCombineAttributes_Empty(t *testing.T) {
	l := NewLogger(LoggerConfig{})
	attrs := l.combineAttributes(context.Background())
	if len(attrs) != 0 {
		t.Fatalf("expected empty, got %v", attrs)
	}
}

func TestCombineAttributes_WithFixedKV(t *testing.T) {
	l := NewLogger(LoggerConfig{FixedKeyValues: map[string]any{"env": "prod"}})
	attrs := l.combineAttributes(context.Background())
	if len(attrs) != 2 { // "env", "prod"
		t.Fatalf("expected 2 items, got %v", attrs)
	}
}

func TestCombineAttributes_WithRequestID(t *testing.T) {
	l := NewLogger(LoggerConfig{})
	ctx := l.SetRequestID(context.Background(), "req-1")
	attrs := l.combineAttributes(ctx)
	// Should contain "request_id", "req-1"
	found := false
	for i := 0; i < len(attrs)-1; i += 2 {
		if attrs[i] == "request_id" && attrs[i+1] == "req-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected request_id in attributes, got %v", attrs)
	}
}

func TestCombineAttributes_WithExtraKV(t *testing.T) {
	l := NewLogger(LoggerConfig{})
	attrs := l.combineAttributes(context.Background(), "key1", "val1")
	// Should contain "key1", "val1"
	if len(attrs) < 2 {
		t.Fatalf("expected keysAndValues, got %v", attrs)
	}
	if attrs[len(attrs)-2] != "key1" || attrs[len(attrs)-1] != "val1" {
		t.Fatalf("expected key1/val1 at end, got %v", attrs)
	}
}

func TestCombineAttributes_AllCombined(t *testing.T) {
	l := NewLogger(LoggerConfig{
		FixedKeyValues: map[string]any{"svc": "api"},
		ExtraFields:    []string{"trace"},
	})
	ctx := l.SetRequestID(context.Background(), "r-1")
	ctx = context.WithValue(ctx, "trace", "t-1")

	attrs := l.combineAttributes(ctx, "custom", "val")
	// Should have: svc/api, request_id/r-1, trace/t-1, custom/val = 8 items
	if len(attrs) != 8 {
		t.Fatalf("expected 8 items, got %d: %v", len(attrs), attrs)
	}
}

// ---------------------------------------------------------------------------
// Instance log methods (no-panic smoke tests)
// ---------------------------------------------------------------------------

func TestLogger_LogMethods_NoPanic(t *testing.T) {
	l := NewLogger(LoggerConfig{DebugLogLevel: true})
	ctx := context.Background()

	// These should not panic. We can't easily capture output without
	// injecting a custom core, so just verify they don't crash.
	l.Debug(ctx, "debug msg")
	l.Info(ctx, "info msg")
	l.Warn(ctx, "warn msg")
	l.Error(ctx, "error msg")
	// Skip Panic and Fatal: they'd abort the test.

	l.Debugf(ctx, "formatted %s %d", "debug", 1)
	l.Infof(ctx, "formatted %s %d", "info", 2)
	l.Warnf(ctx, "formatted %s %d", "warn", 3)
	l.Errorf(ctx, "formatted %s %d", "error", 4)

	l.Debugw(ctx, "structured debug", "key", "val")
	l.Infow(ctx, "structured info", "key", "val")
	l.Warnw(ctx, "structured warn", "key", "val")
	l.Errorw(ctx, "structured error", "key", "val")

	l.Flush()
}

// ---------------------------------------------------------------------------
// Global functions (smoke tests)
// ---------------------------------------------------------------------------

func TestGlobal_InitAndMethods(t *testing.T) {
	InitGlobalLogger(LoggerConfig{
		DebugLogLevel:   true,
		RequestIDPrefix: "G-",
	})

	if !IsDebugLogLevel() {
		t.Fatal("expected debug log level globally")
	}
	if GetRequestIDPrefix() != "G-" {
		t.Fatalf("expected G-, got %q", GetRequestIDPrefix())
	}

	ctx := context.Background()
	Debug(ctx, "global debug")
	Info(ctx, "global info")
	Warn(ctx, "global warn")
	Error(ctx, "global error")

	Debugf(ctx, "global %s", "debugf")
	Infof(ctx, "global %s", "infof")
	Warnf(ctx, "global %s", "warnf")
	Errorf(ctx, "global %s", "errorf")

	Debugw(ctx, "global debugw", "k", 1)
	Infow(ctx, "global infow", "k", 2)
	Warnw(ctx, "global warnw", "k", 3)
	Errorw(ctx, "global errorw", "k", 4)

	Flush()
}

func TestGlobal_SetGetRequestID(t *testing.T) {
	InitGlobalLogger(LoggerConfig{})
	ctx := SetRequestID(context.Background(), "global-req")
	id, ok := GetRequestID(ctx)
	if !ok || id != "global-req" {
		t.Fatalf("expected global-req, got %q ok=%v", id, ok)
	}
}

func TestGlobal_GenerateRequestID(t *testing.T) {
	InitGlobalLogger(LoggerConfig{RequestIDPrefix: "T-"})
	id := GenerateRequestID()
	if !strings.HasPrefix(id, "T-") {
		t.Fatalf("expected T- prefix, got %q", id)
	}
}

func TestGlobal_GetExtraFields(t *testing.T) {
	InitGlobalLogger(LoggerConfig{ExtraFields: []string{"org"}})
	ctx := context.WithValue(context.Background(), "org", "acme")
	fields, ok := GetExtraFields(ctx)
	if !ok {
		t.Fatal("expected true")
	}
	if fields["org"] != "acme" {
		t.Fatalf("expected acme, got %v", fields["org"])
	}
}

func TestGlobal_GetFixedKeyValues(t *testing.T) {
	InitGlobalLogger(LoggerConfig{FixedKeyValues: map[string]any{"region": "us-east"}})
	fkv := GetFixedKeyValues()
	if fkv["region"] != "us-east" {
		t.Fatalf("expected us-east, got %v", fkv["region"])
	}
}

func TestGlobal_GetExtraFieldsList(t *testing.T) {
	InitGlobalLogger(LoggerConfig{ExtraFields: []string{"a", "b"}})
	list := GetExtraFieldsList()
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
}
