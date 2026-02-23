package middleware

import (
	"errors"
	"fmt"
	"net/http"

	httpresponse "github.com/cyrus-wg/gobox/pkg/http_response"
	"github.com/cyrus-wg/gobox/pkg/logger"
)

func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer func() {
			if err := recover(); err != nil {
				logger.Errorw(ctx, "Recovered from panic", "panic", err)

				var panicErr error
				switch v := err.(type) {
				case error:
					panicErr = v
				case string:
					panicErr = errors.New(v)
				default:
					panicErr = fmt.Errorf("panic: %v", v)
				}

				httpresponse.SendErrorJSONResponse(ctx, w, panicErr)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
