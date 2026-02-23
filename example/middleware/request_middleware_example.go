package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"github.com/cyrus-wg/gobox/pkg/middleware"
)

func main() {
	// Initialise the global logger with debug enabled so all log lines are visible.
	logger.InitGlobalLogger(logger.LoggerConfig{
		DebugLogLevel:   true,
		RequestIDPrefix: "req-",
		FixedKeyValues: map[string]any{
			"app": "gobox-example",
		},
	})

	case1_LogRequestDetailsAndLatency()
	case2_LogLatencyOnly()
	case3_NoLogging()
	case4_BypassByAntPattern()
	case5_BypassByRegex()
	case6_BypassByMethod()
	case7_BypassMultipleRules()

	logger.Flush()
}

// ---------------------------------------------------------------------------
// Shared helper
// ---------------------------------------------------------------------------

// newHandler returns a simple HTTP handler that sleeps 5 ms to simulate work,
// then writes a 200 OK response. The sleep makes latency logging observable.
func newHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
}

// send fires a synthetic HTTP request through the given handler and prints a
// short label so the example output is easy to follow.
func send(label string, handler http.Handler, method, path string, headers map[string]string) {
	fmt.Printf("\n  --> %s  [%s %s]\n", label, method, path)

	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

// printSeparator prints a section header.
func printSeparator(title string) {
	fmt.Printf("\n=== %s ===\n", title)
}

// ---------------------------------------------------------------------------
// Case 1 – log request details + latency (full logging)
// ---------------------------------------------------------------------------

// case1_LogRequestDetailsAndLatency shows the most verbose setup:
// every request is logged with its details and the handler latency.
//
// Usage:
//
//	middleware.RequestMiddleware(
//	    true,  // logRequestDetails – log method, path, headers, IP, etc.
//	    true,  // logCompleteTime  – log handler latency in ms after response
//	)
func case1_LogRequestDetailsAndLatency() {
	printSeparator("Case 1: log request details + latency")

	handler := middleware.RequestMiddleware(true, true)(newHandler())

	// Plain browser-like request.
	send("standard GET", handler, http.MethodGet, "/api/products", map[string]string{
		"User-Agent":      "Mozilla/5.0",
		"Accept":          "application/json",
		"Accept-Language": "en-US",
	})

	// Request arriving through a load balancer; middleware reads X-Forwarded-For
	// and X-Forwarded-Proto to expose the real client IP and protocol.
	send("proxied POST (X-Forwarded-For)", handler, http.MethodPost, "/api/orders", map[string]string{
		"Content-Type":      "application/json",
		"X-Forwarded-For":   "203.0.113.42, 10.0.0.1",
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "example.com",
	})
}

// ---------------------------------------------------------------------------
// Case 2 – latency only (no request details)
// ---------------------------------------------------------------------------

// case2_LogLatencyOnly shows how to suppress the verbose request details log
// while still recording how long each handler took.
//
// Usage:
//
//	middleware.RequestMiddleware(
//	    false, // logRequestDetails – skip the incoming-request log line
//	    true,  // logCompleteTime  – still log latency after response
//	)
func case2_LogLatencyOnly() {
	printSeparator("Case 2: latency only (no request details)")

	handler := middleware.RequestMiddleware(false, true)(newHandler())

	send("GET /health", handler, http.MethodGet, "/health", nil)
}

// ---------------------------------------------------------------------------
// Case 3 – no logging at all
// ---------------------------------------------------------------------------

// case3_NoLogging shows the silent mode: the middleware still injects a
// request ID into the context (available to downstream handlers via
// logger.GetRequestID), but emits no log lines itself.
//
// Usage:
//
//	middleware.RequestMiddleware(
//	    false, // logRequestDetails – off
//	    false, // logCompleteTime  – off
//	)
func case3_NoLogging() {
	printSeparator("Case 3: no logging (request ID still injected into context)")

	handler := middleware.RequestMiddleware(false, false)(newHandler())

	send("GET /silent", handler, http.MethodGet, "/silent", nil)
	fmt.Println("  (no log output expected – request ID is still set on context)")
}

// ---------------------------------------------------------------------------
// Case 4 – bypass logging via Ant-style path pattern
// ---------------------------------------------------------------------------

// case4_BypassByAntPattern demonstrates the Ant-style wildcard bypass.
//
// Supported wildcards:
//   - ?  matches exactly one character in a path segment
//   - *  matches zero or more characters within a single path segment
//   - ** matches zero or more path segments (i.e. recursive subtree)
//
// Usage:
//
//	middleware.RequestMiddleware(true, true,
//	    middleware.BypassRequestLogging{Path: "/health"},          // exact path
//	    middleware.BypassRequestLogging{Path: "/metrics/*"},       // one segment wildcard
//	    middleware.BypassRequestLogging{Path: "/internal/**"},     // recursive subtree
//	    middleware.BypassRequestLogging{Path: "/api/v?/ping"},     // single-char wildcard
//	)
func case4_BypassByAntPattern() {
	printSeparator("Case 4: bypass via Ant-style path pattern")

	handler := middleware.RequestMiddleware(true, true,
		// Exact path match – suppress health-check noise.
		middleware.BypassRequestLogging{Path: "/health"},
		// Single-segment wildcard – suppresses /metrics/<anything>.
		middleware.BypassRequestLogging{Path: "/metrics/*"},
		// Recursive wildcard – suppresses everything under /internal/.
		middleware.BypassRequestLogging{Path: "/internal/**"},
		// Single-character wildcard – suppresses /api/v1/ping and /api/v2/ping.
		middleware.BypassRequestLogging{Path: "/api/v?/ping"},
	)(newHandler())

	// These paths match a bypass rule → NO log output.
	send("bypassed  – exact match        /health", handler, http.MethodGet, "/health", nil)
	send("bypassed  – single wildcard    /metrics/cpu", handler, http.MethodGet, "/metrics/cpu", nil)
	send("bypassed  – recursive wildcard /internal/debug/pprof", handler, http.MethodGet, "/internal/debug/pprof", nil)
	send("bypassed  – char wildcard      /api/v1/ping", handler, http.MethodGet, "/api/v1/ping", nil)

	// This path does NOT match any rule → log output IS produced.
	send("NOT bypassed                   /api/v1/products", handler, http.MethodGet, "/api/v1/products", nil)
}

// ---------------------------------------------------------------------------
// Case 5 – bypass logging via regular expression
// ---------------------------------------------------------------------------

// case5_BypassByRegex demonstrates regex-based path bypass.
// Set IsRegex: true and provide a standard Go regex in Path.
// The pattern is automatically anchored (^...$).
//
// Usage:
//
//	middleware.RequestMiddleware(true, true,
//	    middleware.BypassRequestLogging{
//	        Path:    `/api/v\d+/internal/.*`,
//	        IsRegex: true,
//	    },
//	)
func case5_BypassByRegex() {
	printSeparator("Case 5: bypass via regex path pattern")

	handler := middleware.RequestMiddleware(true, true,
		// Suppress any versioned internal API endpoint.
		middleware.BypassRequestLogging{
			Path:    `/api/v\d+/internal/.*`,
			IsRegex: true,
		},
		// Suppress UUIDs in paths (e.g. avoid logging individual resource IDs).
		middleware.BypassRequestLogging{
			Path:    `/api/resource/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			IsRegex: true,
		},
	)(newHandler())

	// Match versioned internal pattern → bypassed.
	send("bypassed  – regex v\\d+/internal /api/v3/internal/cache", handler, http.MethodGet, "/api/v3/internal/cache", nil)

	// Match UUID pattern → bypassed.
	send("bypassed  – UUID path           /api/resource/550e8400-e29b-41d4-a716-446655440000",
		handler, http.MethodGet, "/api/resource/550e8400-e29b-41d4-a716-446655440000", nil)

	// Does NOT match either regex → logged.
	send("NOT bypassed                    /api/v3/products", handler, http.MethodGet, "/api/v3/products", nil)
}

// ---------------------------------------------------------------------------
// Case 6 – bypass logging for specific HTTP methods only
// ---------------------------------------------------------------------------

// case6_BypassByMethod shows how to suppress logging only for certain HTTP
// methods on a path while still logging other methods on the same path.
//
// The Methods field accepts a comma-separated list of HTTP method names
// (case-insensitive). An empty Methods string means all methods.
//
// Usage:
//
//	middleware.RequestMiddleware(true, true,
//	    middleware.BypassRequestLogging{
//	        Path:    "/api/products/**",
//	        Methods: "GET",      // suppress read traffic only
//	    },
//	    middleware.BypassRequestLogging{
//	        Path:    "/api/status",
//	        Methods: "GET,HEAD", // suppress both GET and HEAD
//	    },
//	)
func case6_BypassByMethod() {
	printSeparator("Case 6: bypass by HTTP method")

	handler := middleware.RequestMiddleware(true, true,
		// Suppress read-only polling; still log writes.
		middleware.BypassRequestLogging{
			Path:    "/api/products/**",
			Methods: "GET",
		},
		// Suppress liveness probes on both GET and HEAD.
		middleware.BypassRequestLogging{
			Path:    "/api/status",
			Methods: "GET,HEAD",
		},
	)(newHandler())

	// GET on a bypass path → suppressed.
	send("bypassed  – GET on bypass path  /api/products/123", handler, http.MethodGet, "/api/products/123", nil)

	// POST on the same path → logged (method doesn't match).
	send("NOT bypassed – POST             /api/products/123", handler, http.MethodPost, "/api/products/123", nil)

	// HEAD bypassed on /api/status.
	send("bypassed  – HEAD /api/status", handler, http.MethodHead, "/api/status", nil)

	// DELETE on /api/status → logged.
	send("NOT bypassed – DELETE /api/status", handler, http.MethodDelete, "/api/status", nil)
}

// ---------------------------------------------------------------------------
// Case 7 – multiple bypass rules (mixed patterns and methods)
// ---------------------------------------------------------------------------

// case7_BypassMultipleRules shows a realistic production-like bypass list
// combining exact paths, Ant wildcards, regex patterns, and method filters.
//
// A request is skipped when ANY single rule matches (path AND method).
func case7_BypassMultipleRules() {
	printSeparator("Case 7: multiple bypass rules (mixed)")

	handler := middleware.RequestMiddleware(true, true,

		// 1. Health / readiness probes – all methods, exact match.
		middleware.BypassRequestLogging{Path: "/health"},
		middleware.BypassRequestLogging{Path: "/ready"},

		// 2. Prometheus scrape endpoint – GET only.
		middleware.BypassRequestLogging{Path: "/metrics", Methods: "GET"},

		// 3. Static assets subtree – suppress all reads.
		middleware.BypassRequestLogging{Path: "/static/**", Methods: "GET,HEAD"},

		// 4. Admin internal tools – regex, any method.
		middleware.BypassRequestLogging{
			Path:    `/admin/(debug|pprof|trace)(/.*)?`,
			IsRegex: true,
		},
	)(newHandler())

	// --- Requests that are bypassed ---
	send("bypassed  /health", handler, http.MethodGet, "/health", nil)
	send("bypassed  /ready", handler, http.MethodGet, "/ready", nil)
	send("bypassed  /metrics GET", handler, http.MethodGet, "/metrics", nil)
	send("bypassed  /static/css/app.css GET", handler, http.MethodGet, "/static/css/app.css", nil)
	send("bypassed  /admin/debug GET", handler, http.MethodGet, "/admin/debug", nil)
	send("bypassed  /admin/pprof/heap GET", handler, http.MethodGet, "/admin/pprof/heap", nil)

	// --- Requests that are NOT bypassed ---
	send("NOT bypassed  /metrics POST", handler, http.MethodPost, "/metrics", nil)       // wrong method for rule 2
	send("NOT bypassed  /static/css POST", handler, http.MethodPost, "/static/css", nil) // wrong method for rule 3
	send("NOT bypassed  /api/users GET", handler, http.MethodGet, "/api/users", nil)     // no matching rule
}
