package middleware

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"github.com/cyrus-wg/gobox/pkg/pattern"
)

func RequestMiddleware(logRequestDetails bool, logCompleteTime bool, bypassList ...BypassRequestLogging) func(next http.Handler) http.Handler {
	compiledBypassList := compileBypassPatterns(bypassList)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			requestId := logger.GenerateRequestID()
			r = r.WithContext(logger.SetRequestID(r.Context(), requestId))

			shouldSkipLogging := shouldBypassMiddlewareLogging(compiledBypassList, r.URL.Path, r.Method)

			if logRequestDetails && !shouldSkipLogging {
				requestData := map[string]any{
					// Basic request info
					"method":       r.Method,
					"url":          r.URL.String(),
					"path":         r.URL.Path,
					"query_params": r.URL.RawQuery,
					"protocol":     r.Proto,
					"host":         r.Host,

					// Client information
					"user_ip":     getRealUserIP(r),
					"remote_addr": r.RemoteAddr,
					"user_agent":  r.Header.Get("User-Agent"),
					"referer":     r.Header.Get("Referer"),

					// Request size and content
					"content_type":    r.Header.Get("Content-Type"),
					"content_length":  r.ContentLength,
					"accept":          r.Header.Get("Accept"),
					"accept_encoding": r.Header.Get("Accept-Encoding"),
					"accept_language": r.Header.Get("Accept-Language"),

					// Security headers
					"origin": r.Header.Get("Origin"),

					// Load balancer / proxy headers
					"x_forwarded_for":   r.Header.Get("X-Forwarded-For"),
					"x_forwarded_proto": r.Header.Get("X-Forwarded-Proto"),
					"x_forwarded_host":  r.Header.Get("X-Forwarded-Host"),
					"x_real_ip":         r.Header.Get("X-Real-IP"),
					"x_client_ip":       r.Header.Get("X-Client-IP"),
				}

				logger.Infow(r.Context(), "Incoming request", "details", requestData)
			}

			next.ServeHTTP(w, r)

			latency := time.Since(startTime)

			if logCompleteTime && !shouldSkipLogging {
				logger.Infow(r.Context(), "Request completed", "latency_ms", latency.Milliseconds())
			}
		})
	}
}

type BypassRequestLogging struct {
	Path    string
	Methods string         // Comma-separated methods (e.g., "GET,POST"), empty means all methods
	IsRegex bool           // If true, Path is treated as a regex pattern; otherwise Ant-style pattern
	regex   *regexp.Regexp // Pre-compiled regex (internal use)
}

// compileBypassPatterns pre-compiles regex patterns automatically.
// This is called internally when middleware is initialized.
func compileBypassPatterns(patterns []BypassRequestLogging) []BypassRequestLogging {
	compiled := make([]BypassRequestLogging, len(patterns))
	for i, p := range patterns {
		compiled[i] = p
		if p.IsRegex {
			if re, err := regexp.Compile("^" + p.Path + "$"); err == nil {
				compiled[i].regex = re
			}
		}
	}
	return compiled
}

func shouldBypassMiddlewareLogging(bypassList []BypassRequestLogging, path string, method string) bool {
	for _, bypass := range bypassList {
		if !matchMethod(bypass.Methods, method) {
			continue
		}

		if bypass.IsRegex {
			if matchRegex(&bypass, path) {
				return true
			}
		} else {
			if pattern.MatchAnt(bypass.Path, path) {
				return true
			}
		}
	}

	return false
}

// matchMethod checks if the request method matches the allowed methods.
// Empty methods string means all methods are allowed.
func matchMethod(methods string, method string) bool {
	if methods == "" {
		return true
	}

	// Fast path for single method (no comma)
	if !strings.Contains(methods, ",") {
		return strings.EqualFold(strings.TrimSpace(methods), method)
	}

	// Split and check each method
	for m := range strings.SplitSeq(methods, ",") {
		if strings.EqualFold(strings.TrimSpace(m), method) {
			return true
		}
	}
	return false
}

// matchRegex matches path against a regex pattern
// Uses pre-compiled regex if available, otherwise compiles on the fly
func matchRegex(bypass *BypassRequestLogging, path string) bool {
	// Use pre-compiled regex if available
	if bypass.regex != nil {
		return bypass.regex.MatchString(path)
	}

	// Compile regex on the fly if not pre-compiled (fallback)
	return pattern.MatchRegex(bypass.Path, path)
}
