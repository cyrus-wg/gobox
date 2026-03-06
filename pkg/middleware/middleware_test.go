package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httperror "github.com/cyrus-wg/gobox/pkg/http_error"
	"github.com/cyrus-wg/gobox/pkg/logger"
)

func init() {
	logger.InitGlobalLogger(logger.LoggerConfig{})
	_ = httperror.SetErrorIDLength(0)
}

// ---------------------------------------------------------------------------
// DecodeRequestBodyMiddleware
// ---------------------------------------------------------------------------

type testPayload struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0"`
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestDecodeRequestBodyMiddleware_ValidJSON(t *testing.T) {
	body := `{"name":"Alice","email":"alice@example.com","age":30}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	var captured *testPayload
	handler := DecodeRequestBodyMiddleware[testPayload]()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := GetRequestBodyFromContext[testPayload](r)
			if !ok {
				t.Fatal("request body not in context")
			}
			captured = p
			w.WriteHeader(200)
		}),
	)
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if captured == nil || captured.Name != "Alice" {
		t.Fatalf("expected captured name Alice, got %+v", captured)
	}
}

func TestDecodeRequestBodyMiddleware_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()

	handler := DecodeRequestBodyMiddleware[testPayload]()(okHandler())
	handler.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "DECODE_BODY_ERROR") {
		t.Fatalf("expected DECODE_BODY_ERROR, got: %s", w.Body.String())
	}
}

func TestDecodeRequestBodyMiddleware_ValidationFailure(t *testing.T) {
	// Missing required "email" field
	body := `{"name":"Bob","age":25}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler := DecodeRequestBodyMiddleware[testPayload]()(okHandler())
	handler.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "REQUEST_BODY_CONSTRAINT_VIOLATION") {
		t.Fatalf("expected constraint violation, got: %s", w.Body.String())
	}
}

func TestDecodeRequestBodyMiddleware_EmptyBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(""))
	w := httptest.NewRecorder()

	handler := DecodeRequestBodyMiddleware[testPayload]()(okHandler())
	handler.ServeHTTP(w, req)

	// Empty body should fail JSON unmarshal
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDecodeRequestBodyMiddleware_LogRequestBodyFalse(t *testing.T) {
	body := `{"name":"Test","email":"test@test.com","age":1}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler := DecodeRequestBodyMiddleware[testPayload](false)(okHandler())
	handler.ServeHTTP(w, req)

	// Should still succeed, just without logging
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetRequestBodyFromContext_Missing(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	_, ok := GetRequestBodyFromContext[testPayload](req)
	if ok {
		t.Fatal("expected false when body not in context")
	}
}

// ---------------------------------------------------------------------------
// RecoverMiddleware
// ---------------------------------------------------------------------------

func TestRecoverMiddleware_NoPanic(t *testing.T) {
	handler := RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("fine"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRecoverMiddleware_PanicString(t *testing.T) {
	handler := RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestRecoverMiddleware_PanicError(t *testing.T) {
	handler := RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(io.ErrUnexpectedEOF)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestRecoverMiddleware_PanicInt(t *testing.T) {
	handler := RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(42)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// RequestMiddleware
// ---------------------------------------------------------------------------

func TestRequestMiddleware_SetsRequestID(t *testing.T) {
	var capturedID string
	handler := RequestMiddleware(false, false)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := logger.GetRequestID(r.Context())
			if !ok {
				t.Fatal("request ID not found in context")
			}
			capturedID = id
			w.WriteHeader(200)
		}),
	)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedID == "" {
		t.Fatal("expected non-empty request ID to be set in context")
	}
}

func TestRequestMiddleware_LogsRequestDetails(t *testing.T) {
	handler := RequestMiddleware(true, true)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		}),
	)

	req := httptest.NewRequest("GET", "/api/test?q=1", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequestMiddleware_BypassAntPattern(t *testing.T) {
	var logged bool
	handler := RequestMiddleware(true, true, BypassRequestLogging{
		Path: "/health",
	})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If bypass works, the middleware skips logging but still calls next
			logged = true
			w.WriteHeader(200)
		}),
	)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !logged {
		t.Fatal("handler should still be called even if logging is bypassed")
	}
}

func TestRequestMiddleware_BypassRegexPattern(t *testing.T) {
	handler := RequestMiddleware(true, true, BypassRequestLogging{
		Path:    "/health.*",
		IsRegex: true,
	})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		}),
	)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequestMiddleware_BypassWithMethod(t *testing.T) {
	handler := RequestMiddleware(true, true, BypassRequestLogging{
		Path:    "/api/ping",
		Methods: "GET",
	})(okHandler())

	// GET should be bypassed
	req := httptest.NewRequest("GET", "/api/ping", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 for GET, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// shouldBypassMiddlewareLogging
// ---------------------------------------------------------------------------

func TestShouldBypassMiddlewareLogging_EmptyList(t *testing.T) {
	if shouldBypassMiddlewareLogging(nil, "/anything", "GET") {
		t.Fatal("empty list should not bypass")
	}
}

func TestShouldBypassMiddlewareLogging_AntMatch(t *testing.T) {
	list := compileBypassPatterns([]BypassRequestLogging{
		{Path: "/api/v1/**"},
	})
	if !shouldBypassMiddlewareLogging(list, "/api/v1/users/123", "GET") {
		t.Fatal("expected bypass for /api/v1/**")
	}
	if shouldBypassMiddlewareLogging(list, "/api/v2/users", "GET") {
		t.Fatal("should not bypass /api/v2")
	}
}

func TestShouldBypassMiddlewareLogging_MethodFilter(t *testing.T) {
	list := compileBypassPatterns([]BypassRequestLogging{
		{Path: "/metrics", Methods: "GET,HEAD"},
	})
	if !shouldBypassMiddlewareLogging(list, "/metrics", "GET") {
		t.Fatal("GET should match")
	}
	if !shouldBypassMiddlewareLogging(list, "/metrics", "HEAD") {
		t.Fatal("HEAD should match")
	}
	if shouldBypassMiddlewareLogging(list, "/metrics", "POST") {
		t.Fatal("POST should not match")
	}
}

// ---------------------------------------------------------------------------
// matchMethod
// ---------------------------------------------------------------------------

func TestMatchMethod(t *testing.T) {
	tests := []struct {
		methods string
		method  string
		want    bool
	}{
		{"", "GET", true},           // empty = all
		{"GET", "GET", true},        // exact
		{"get", "GET", true},        // case insensitive
		{"GET,POST", "POST", true},  // multi
		{"GET, POST", "POST", true}, // multi with space
		{"GET,POST", "DELETE", false},
		{"PUT", "GET", false},
	}
	for _, tt := range tests {
		if got := matchMethod(tt.methods, tt.method); got != tt.want {
			t.Errorf("matchMethod(%q, %q) = %v, want %v", tt.methods, tt.method, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// getRealUserIP
// ---------------------------------------------------------------------------

func TestGetRealUserIP(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(r *http.Request)
		wantIP string
	}{
		{
			name: "X-Forwarded-For single",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "1.2.3.4")
			},
			wantIP: "1.2.3.4",
		},
		{
			name: "X-Forwarded-For chain",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
			},
			wantIP: "1.2.3.4",
		},
		{
			name: "X-Real-IP",
			setup: func(r *http.Request) {
				r.Header.Set("X-Real-IP", "10.0.0.1")
			},
			wantIP: "10.0.0.1",
		},
		{
			name: "X-Client-IP",
			setup: func(r *http.Request) {
				r.Header.Set("X-Client-IP", "192.168.1.1")
			},
			wantIP: "192.168.1.1",
		},
		{
			name:   "RemoteAddr with port",
			setup:  func(r *http.Request) { r.RemoteAddr = "172.16.0.1:12345" },
			wantIP: "172.16.0.1",
		},
		{
			name:   "RemoteAddr without port",
			setup:  func(r *http.Request) { r.RemoteAddr = "172.16.0.1" },
			wantIP: "172.16.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			// Clear default headers
			r.Header = http.Header{}
			tt.setup(r)
			if got := getRealUserIP(r); got != tt.wantIP {
				t.Errorf("getRealUserIP() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// compileBypassPatterns
// ---------------------------------------------------------------------------

func TestCompileBypassPatterns_InvalidRegex(t *testing.T) {
	// Invalid regex should not panic, just leave regex nil
	patterns := compileBypassPatterns([]BypassRequestLogging{
		{Path: "[invalid", IsRegex: true},
	})
	if patterns[0].regex != nil {
		t.Fatal("expected nil regex on invalid pattern")
	}
}

// ---------------------------------------------------------------------------
// matchRegex
// ---------------------------------------------------------------------------

func TestMatchRegex_Fallback(t *testing.T) {
	// When regex is nil, it should fallback to pattern.MatchRegex
	b := &BypassRequestLogging{Path: "/api/users/\\d+", IsRegex: true}
	if !matchRegex(b, "/api/users/123") {
		t.Fatal("expected fallback match")
	}
}

// ---------------------------------------------------------------------------
// Chaining middleware
// ---------------------------------------------------------------------------

func TestMiddlewareChaining(t *testing.T) {
	handler := RecoverMiddleware(
		RequestMiddleware(false, false)(
			DecodeRequestBodyMiddleware[testPayload]()(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					p, ok := GetRequestBodyFromContext[testPayload](r)
					if !ok {
						w.WriteHeader(500)
						return
					}
					w.WriteHeader(200)
					_ = json.NewEncoder(w).Encode(p)
				}),
			),
		),
	)

	body := `{"name":"Chain","email":"chain@test.com","age":5}`
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Integration: Recover + DecodeBody panic
// ---------------------------------------------------------------------------

func TestRecoverMiddleware_WithPanicInDecodeChain(t *testing.T) {
	handler := RecoverMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("decode boom")
		}),
	)

	req := httptest.NewRequest("POST", "/", nil)
	// Provide a request context that has requestID (simulate RequestMiddleware)
	req = req.WithContext(logger.SetRequestID(context.Background(), "test-id"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
