package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	defaultClient *http.Client
	defaultMu     sync.RWMutex // protects defaultClient for concurrent access
)

func init() {
	defaultClient = NewClient(nil)
}

// SetDefaultClient replaces the package-level client used by SendRequest and
// all JSON* convenience functions. It is safe to call concurrently.
// Pass nil to reset to the built-in defaults.
//
// Prefer creating an *HTTPClient instance (NewHTTPClient) over replacing the
// global when different parts of your application need different settings.
func SetDefaultClient(client *http.Client) {
	c := NewClient(client)
	defaultMu.Lock()
	defaultClient = c
	defaultMu.Unlock()
}

// getDefaultClient returns the current package-level client under the read
// lock. Callers must not mutate the returned value; use SetDefaultClient to
// replace it.
func getDefaultClient() *http.Client {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultClient
}

// NewClient populates zero/nil fields in client with production-ready defaults
// and returns it. Pass nil to get a fully-defaulted client.
//
// Transport handling:
//   - nil Transport      → a new *http.Transport is created (ForceAttemptHTTP2=true).
//   - *http.Transport    → only zero-value duration/size fields are filled;
//     boolean fields are left untouched so explicit caller choices are kept.
//   - other RoundTripper → used as-is.
func NewClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}

	switch t := client.Transport.(type) {
	case nil:
		client.Transport = DefaultTransport()
	case *http.Transport:
		applyTransportDefaults(t)
	}

	return client
}

// DefaultTransport returns a new *http.Transport with all default settings.
// Use this when wrapping the transport in a custom RoundTripper while keeping
// the baseline configuration:
//
//	client := &http.Client{
//	    Transport: myLoggingRoundTripper{next: httpclient.DefaultTransport()},
//	}
func DefaultTransport() *http.Transport {
	t := &http.Transport{
		ForceAttemptHTTP2: true, // safe: we own this transport
	}
	applyTransportDefaults(t)
	return t
}

// applyTransportDefaults fills in only the zero-value non-boolean fields of t.
// Boolean fields (ForceAttemptHTTP2, DisableKeepAlives, DisableCompression) are
// intentionally skipped: false is a valid explicit caller choice and is
// indistinguishable from the zero value.
func applyTransportDefaults(t *http.Transport) {
	if t.MaxIdleConns == 0 {
		t.MaxIdleConns = 100
	}
	if t.MaxIdleConnsPerHost == 0 {
		t.MaxIdleConnsPerHost = 50
	}
	if t.MaxConnsPerHost == 0 {
		t.MaxConnsPerHost = 100
	}
	if t.IdleConnTimeout == 0 {
		t.IdleConnTimeout = 90 * time.Second
	}
	if t.TLSHandshakeTimeout == 0 {
		t.TLSHandshakeTimeout = 10 * time.Second
	}
	if t.ResponseHeaderTimeout == 0 {
		t.ResponseHeaderTimeout = 30 * time.Second
	}
	if t.ExpectContinueTimeout == 0 {
		t.ExpectContinueTimeout = 1 * time.Second
	}
	if t.MaxResponseHeaderBytes == 0 {
		t.MaxResponseHeaderBytes = 1 << 20 // 1 MB
	}
	if t.DialContext == nil {
		t.DialContext = (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext
	}
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	} else if t.TLSClientConfig.MinVersion == 0 {
		t.TLSClientConfig.MinVersion = tls.VersionTLS12
	}
}

// isJSONContentType reports whether ct is an application/json Content-Type,
// accounting for optional parameters (e.g. "application/json; charset=utf-8").
func isJSONContentType(ct string) bool {
	ct = strings.TrimSpace(strings.ToLower(ct))
	return ct == "application/json" || strings.HasPrefix(ct, "application/json;")
}

// buildBody converts body into an io.Reader based on its runtime type:
//
//   - io.Reader → used directly; Content-Type is irrelevant.
//   - []byte    → wrapped in bytes.NewBuffer.
//   - string    → wrapped in strings.NewReader.
//   - other     → JSON-marshalled via json.Marshal, only when contentType is
//     application/json (with or without parameters); any other Content-Type
//     returns an error.
func buildBody(body any, contentType string) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	switch v := body.(type) {
	case io.Reader:
		return v, nil
	case []byte:
		return bytes.NewBuffer(v), nil
	case string:
		return strings.NewReader(v), nil
	default:
		if !isJSONContentType(contentType) {
			return nil, fmt.Errorf(
				"httpclient: cannot encode body of type %T for Content-Type %q; "+
					"pass an io.Reader, []byte, or string for non-JSON content types",
				body, contentType,
			)
		}
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(data), nil
	}
}

// doRequest is the single shared implementation for all public methods.
// It builds the request body, attaches headers, executes the call via client,
// and returns (statusCode, responseBody, error).
// HTTP status codes >= 400 are surfaced as a non-nil error while still
// returning the raw response body so callers can inspect the error payload.
func doRequest(ctx context.Context, client *http.Client, method, url string, body any, headers map[string]string) (int, []byte, error) {
	reqBody, err := buildBody(body, headers["Content-Type"])
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return 0, nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	if resp.StatusCode >= 400 {
		return resp.StatusCode, respBody, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return resp.StatusCode, respBody, nil
}

// SendRequest executes an HTTP request using the package-level default client.
// ctx controls cancellation and deadlines; pass context.Background() when no
// deadline propagation is needed.
func SendRequest(ctx context.Context, method, url string, body any, headers map[string]string) (int, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return doRequest(ctx, getDefaultClient(), method, url, body, headers)
}

// jsonHeaders returns a shallow copy of headers with Content-Type set to
// "application/json". The original map is never mutated.
func jsonHeaders(headers map[string]string) map[string]string {
	h := make(map[string]string, len(headers)+1)
	maps.Copy(h, headers)
	h["Content-Type"] = "application/json"
	return h
}

func JSONGet(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	return SendRequest(ctx, http.MethodGet, url, nil, jsonHeaders(headers))
}

func JSONPost(ctx context.Context, url string, body any, headers map[string]string) (int, []byte, error) {
	return SendRequest(ctx, http.MethodPost, url, body, jsonHeaders(headers))
}

func JSONPut(ctx context.Context, url string, body any, headers map[string]string) (int, []byte, error) {
	return SendRequest(ctx, http.MethodPut, url, body, jsonHeaders(headers))
}

func JSONPatch(ctx context.Context, url string, body any, headers map[string]string) (int, []byte, error) {
	return SendRequest(ctx, http.MethodPatch, url, body, jsonHeaders(headers))
}

func JSONDelete(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	return SendRequest(ctx, http.MethodDelete, url, nil, jsonHeaders(headers))
}

func JSONOptions(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	return SendRequest(ctx, http.MethodOptions, url, nil, jsonHeaders(headers))
}

func JSONHead(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	return SendRequest(ctx, http.MethodHead, url, nil, jsonHeaders(headers))
}

// HTTPClient wraps an *http.Client and mirrors the package-level API.
// Use this when different parts of your application need separate clients
// (e.g. different timeouts or TLS config) rather than sharing the global.
//
//	c := httpclient.NewHTTPClient(&http.Client{Timeout: 5 * time.Minute})
//	status, body, err := c.JSONPost(ctx, url, payload, nil)
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient wraps client with production defaults applied via NewClient.
// Pass nil to start from all defaults.
func NewHTTPClient(client *http.Client) *HTTPClient {
	return &HTTPClient{client: NewClient(client)}
}

// SendRequest executes an HTTP request using this instance's client.
// ctx controls cancellation and deadlines; pass context.Background() when no
// deadline propagation is needed.
func (hc *HTTPClient) SendRequest(ctx context.Context, method, url string, body any, headers map[string]string) (int, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return doRequest(ctx, hc.client, method, url, body, headers)
}

func (hc *HTTPClient) JSONGet(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	return hc.SendRequest(ctx, http.MethodGet, url, nil, jsonHeaders(headers))
}

func (hc *HTTPClient) JSONPost(ctx context.Context, url string, body any, headers map[string]string) (int, []byte, error) {
	return hc.SendRequest(ctx, http.MethodPost, url, body, jsonHeaders(headers))
}

func (hc *HTTPClient) JSONPut(ctx context.Context, url string, body any, headers map[string]string) (int, []byte, error) {
	return hc.SendRequest(ctx, http.MethodPut, url, body, jsonHeaders(headers))
}

func (hc *HTTPClient) JSONPatch(ctx context.Context, url string, body any, headers map[string]string) (int, []byte, error) {
	return hc.SendRequest(ctx, http.MethodPatch, url, body, jsonHeaders(headers))
}

func (hc *HTTPClient) JSONDelete(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	return hc.SendRequest(ctx, http.MethodDelete, url, nil, jsonHeaders(headers))
}

func (hc *HTTPClient) JSONOptions(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	return hc.SendRequest(ctx, http.MethodOptions, url, nil, jsonHeaders(headers))
}

func (hc *HTTPClient) JSONHead(ctx context.Context, url string, headers map[string]string) (int, []byte, error) {
	return hc.SendRequest(ctx, http.MethodHead, url, nil, jsonHeaders(headers))
}
