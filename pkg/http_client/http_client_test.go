package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// echoServer returns an httptest.Server that echoes back method, headers,
// and body in a JSON response.
func echoServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"method":       r.Method,
			"content_type": r.Header.Get("Content-Type"),
			"body":         string(body),
			"x_custom":     r.Header.Get("X-Custom"),
		})
	}))
}

// statusServer returns a server that always responds with the given status code.
func statusServer(code int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"error":"forced"}`))
	}))
}

// parseEcho unmarshals the echo server JSON response.
func parseEcho(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to parse echo: %v\nraw: %s", err, data)
	}
	return m
}

// ---------------------------------------------------------------------------
// NewClient / DefaultTransport
// ---------------------------------------------------------------------------

func TestNewClient_NilGetsDefaults(t *testing.T) {
	c := NewClient(nil)
	if c.Timeout != 30*time.Second {
		t.Fatalf("expected 30s timeout, got %v", c.Timeout)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if tr.MaxIdleConns != 100 {
		t.Fatalf("expected MaxIdleConns=100, got %d", tr.MaxIdleConns)
	}
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("expected TLS 1.2 minimum")
	}
}

func TestNewClient_PreservesExistingTimeout(t *testing.T) {
	c := NewClient(&http.Client{Timeout: 5 * time.Second})
	if c.Timeout != 5*time.Second {
		t.Fatalf("expected 5s, got %v", c.Timeout)
	}
}

func TestNewClient_PreservesCustomTransport(t *testing.T) {
	custom := &http.Transport{MaxIdleConns: 7}
	c := NewClient(&http.Client{Transport: custom})
	tr := c.Transport.(*http.Transport)
	if tr.MaxIdleConns != 7 {
		t.Fatalf("expected 7 (caller's choice), got %d", tr.MaxIdleConns)
	}
	// Zero-value fields should be filled
	if tr.MaxIdleConnsPerHost != 50 {
		t.Fatalf("expected MaxIdleConnsPerHost=50, got %d", tr.MaxIdleConnsPerHost)
	}
}

func TestNewClient_NonHTTPTransportUntouched(t *testing.T) {
	// A custom RoundTripper that is NOT *http.Transport should be preserved as-is.
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})
	c := NewClient(&http.Client{Transport: rt})
	if _, ok := c.Transport.(roundTripFunc); !ok {
		t.Fatal("custom RoundTripper should be left untouched")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDefaultTransport_ReturnsDefaults(t *testing.T) {
	tr := DefaultTransport()
	if !tr.ForceAttemptHTTP2 {
		t.Fatal("expected ForceAttemptHTTP2=true")
	}
	if tr.MaxIdleConns != 100 {
		t.Fatalf("expected 100, got %d", tr.MaxIdleConns)
	}
}

// ---------------------------------------------------------------------------
// isJSONContentType
// ---------------------------------------------------------------------------

func TestIsJSONContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"application/json", true},
		{"Application/JSON", true},
		{"application/json; charset=utf-8", true},
		{"text/plain", false},
		{"", false},
		{"application/xml", false},
	}
	for _, tt := range tests {
		if got := isJSONContentType(tt.ct); got != tt.want {
			t.Errorf("isJSONContentType(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// buildBody
// ---------------------------------------------------------------------------

func TestBuildBody_Nil(t *testing.T) {
	r, err := buildBody(nil, "")
	if err != nil || r != nil {
		t.Fatal("nil body should yield nil reader")
	}
}

func TestBuildBody_IoReader(t *testing.T) {
	in := strings.NewReader("hello")
	r, err := buildBody(in, "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if r != in {
		t.Fatal("expected same io.Reader back")
	}
}

func TestBuildBody_ByteSlice(t *testing.T) {
	r, err := buildBody([]byte("data"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	if string(b) != "data" {
		t.Fatalf("expected 'data', got %q", b)
	}
}

func TestBuildBody_String(t *testing.T) {
	r, err := buildBody("payload", "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	if string(b) != "payload" {
		t.Fatalf("expected 'payload', got %q", b)
	}
}

func TestBuildBody_StructWithJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	r, err := buildBody(payload{Name: "test"}, "application/json")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	if !strings.Contains(string(b), `"name":"test"`) {
		t.Fatalf("expected JSON encoding, got %q", b)
	}
}

func TestBuildBody_StructWithNonJSON_Fails(t *testing.T) {
	_, err := buildBody(struct{}{}, "text/plain")
	if err == nil {
		t.Fatal("expected error for non-JSON content type with struct body")
	}
}

// ---------------------------------------------------------------------------
// SendRequest / package-level JSON helpers
// ---------------------------------------------------------------------------

func TestSendRequest_GET(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	status, body, err := SendRequest(context.Background(), "GET", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	m := parseEcho(t, body)
	if m["method"] != "GET" {
		t.Fatalf("expected GET, got %v", m["method"])
	}
}

func TestSendRequest_NilContext(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	status, _, err := SendRequest(nil, "GET", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
}

func TestSendRequest_WithHeaders(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	_, body, err := SendRequest(context.Background(), "GET", srv.URL, nil, map[string]string{
		"X-Custom": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := parseEcho(t, body)
	if m["x_custom"] != "hello" {
		t.Fatalf("expected header, got %v", m["x_custom"])
	}
}

func TestSendRequest_BodyTypes(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	ctx := context.Background()

	// io.Reader
	_, body, _ := SendRequest(ctx, "POST", srv.URL, bytes.NewReader([]byte("reader")),
		map[string]string{"Content-Type": "text/plain"})
	m := parseEcho(t, body)
	if m["body"] != "reader" {
		t.Fatalf("expected reader body, got %v", m["body"])
	}

	// string
	_, body, _ = SendRequest(ctx, "POST", srv.URL, "strval",
		map[string]string{"Content-Type": "text/plain"})
	m = parseEcho(t, body)
	if m["body"] != "strval" {
		t.Fatalf("expected strval, got %v", m["body"])
	}
}

func TestSendRequest_400PlusReturnsError(t *testing.T) {
	srv := statusServer(404)
	defer srv.Close()

	status, body, err := SendRequest(context.Background(), "GET", srv.URL, nil, nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if status != 404 {
		t.Fatalf("expected 404, got %d", status)
	}
	if !strings.Contains(string(body), "forced") {
		t.Fatalf("expected body despite error, got %q", body)
	}
}

func TestSendRequest_InvalidURL(t *testing.T) {
	_, _, err := SendRequest(context.Background(), "GET", "http://[::1]:namedport", nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestJSONGet(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	status, body, err := JSONGet(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	m := parseEcho(t, body)
	if m["method"] != "GET" {
		t.Fatalf("expected GET, got %v", m["method"])
	}
	if ct, ok := m["content_type"].(string); !ok || !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content type, got %v", m["content_type"])
	}
}

func TestJSONPost(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	payload := map[string]string{"key": "value"}
	_, body, err := JSONPost(context.Background(), srv.URL, payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := parseEcho(t, body)
	if m["method"] != "POST" {
		t.Fatalf("expected POST, got %v", m["method"])
	}
	if !strings.Contains(m["body"].(string), `"key":"value"`) {
		t.Fatalf("expected JSON body, got %v", m["body"])
	}
}

func TestJSONPut(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	_, body, err := JSONPut(context.Background(), srv.URL, map[string]int{"n": 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := parseEcho(t, body)
	if m["method"] != "PUT" {
		t.Fatalf("expected PUT, got %v", m["method"])
	}
}

func TestJSONPatch(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	_, body, err := JSONPatch(context.Background(), srv.URL, "raw", nil)
	if err != nil {
		t.Fatal(err)
	}
	m := parseEcho(t, body)
	if m["method"] != "PATCH" {
		t.Fatalf("expected PATCH, got %v", m["method"])
	}
}

func TestJSONDelete(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	_, body, err := JSONDelete(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := parseEcho(t, body)
	if m["method"] != "DELETE" {
		t.Fatalf("expected DELETE, got %v", m["method"])
	}
}

func TestJSONOptions(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	_, body, err := JSONOptions(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := parseEcho(t, body)
	if m["method"] != "OPTIONS" {
		t.Fatalf("expected OPTIONS, got %v", m["method"])
	}
}

func TestJSONHead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	status, _, err := JSONHead(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
}

func TestJSONHelpers_PreserveExtraHeaders(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	hdrs := map[string]string{"X-Custom": "preserved"}
	_, body, err := JSONGet(context.Background(), srv.URL, hdrs)
	if err != nil {
		t.Fatal(err)
	}
	m := parseEcho(t, body)
	if m["x_custom"] != "preserved" {
		t.Fatalf("expected custom header preserved, got %v", m["x_custom"])
	}
}

// ---------------------------------------------------------------------------
// SetDefaultClient
// ---------------------------------------------------------------------------

func TestSetDefaultClient(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	// Set a custom client with a short timeout
	custom := &http.Client{Timeout: 2 * time.Second}
	SetDefaultClient(custom)
	defer SetDefaultClient(nil) // restore

	status, _, err := SendRequest(context.Background(), "GET", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
}

func TestSetDefaultClient_Nil(t *testing.T) {
	SetDefaultClient(nil) // reset to defaults
	c := getDefaultClient()
	if c.Timeout != 30*time.Second {
		t.Fatalf("expected 30s default after nil reset, got %v", c.Timeout)
	}
}

// ---------------------------------------------------------------------------
// HTTPClient instance
// ---------------------------------------------------------------------------

func TestHTTPClient_SendRequest(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	hc := NewHTTPClient(nil)
	status, body, err := hc.SendRequest(context.Background(), "POST", srv.URL, "data",
		map[string]string{"Content-Type": "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	m := parseEcho(t, body)
	if m["method"] != "POST" {
		t.Fatalf("expected POST, got %v", m["method"])
	}
}

func TestHTTPClient_NilContext(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	hc := NewHTTPClient(nil)
	status, _, err := hc.SendRequest(nil, "GET", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
}

func TestHTTPClient_JSONMethods(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	ctx := context.Background()
	hc := NewHTTPClient(nil)

	tests := []struct {
		name   string
		call   func() (int, []byte, error)
		method string
	}{
		{"Get", func() (int, []byte, error) { return hc.JSONGet(ctx, srv.URL, nil) }, "GET"},
		{"Post", func() (int, []byte, error) { return hc.JSONPost(ctx, srv.URL, nil, nil) }, "POST"},
		{"Put", func() (int, []byte, error) { return hc.JSONPut(ctx, srv.URL, nil, nil) }, "PUT"},
		{"Patch", func() (int, []byte, error) { return hc.JSONPatch(ctx, srv.URL, nil, nil) }, "PATCH"},
		{"Delete", func() (int, []byte, error) { return hc.JSONDelete(ctx, srv.URL, nil) }, "DELETE"},
		{"Options", func() (int, []byte, error) { return hc.JSONOptions(ctx, srv.URL, nil) }, "OPTIONS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, body, err := tt.call()
			if err != nil {
				t.Fatal(err)
			}
			m := parseEcho(t, body)
			if m["method"] != tt.method {
				t.Fatalf("expected %s, got %v", tt.method, m["method"])
			}
		})
	}
}

func TestHTTPClient_JSONHead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	hc := NewHTTPClient(nil)
	status, _, err := hc.JSONHead(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 204 {
		t.Fatalf("expected 204, got %d", status)
	}
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestSendRequest_CancelledContext(t *testing.T) {
	srv := echoServer()
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, _, err := SendRequest(ctx, "GET", srv.URL, nil, nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// ---------------------------------------------------------------------------
// jsonHeaders
// ---------------------------------------------------------------------------

func TestJsonHeaders_DoesNotMutateOriginal(t *testing.T) {
	orig := map[string]string{"X-A": "1"}
	h := jsonHeaders(orig)
	if h["Content-Type"] != "application/json" {
		t.Fatal("Content-Type not set")
	}
	if _, ok := orig["Content-Type"]; ok {
		t.Fatal("original map was mutated")
	}
	if h["X-A"] != "1" {
		t.Fatal("extra header lost")
	}
}

func TestJsonHeaders_NilInput(t *testing.T) {
	h := jsonHeaders(nil)
	if h["Content-Type"] != "application/json" {
		t.Fatal("Content-Type not set on nil input")
	}
}
