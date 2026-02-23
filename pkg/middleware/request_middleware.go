package middleware

import (
	"net/http"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
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
