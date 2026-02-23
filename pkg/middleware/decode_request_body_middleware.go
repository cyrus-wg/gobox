package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	httperror "github.com/cyrus-wg/gobox/pkg/http_error"
	httpresponse "github.com/cyrus-wg/gobox/pkg/http_response"
	"github.com/cyrus-wg/gobox/pkg/logger"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

func DecodeRequestBodyMiddleware[T any](logRequestBody ...bool) func(next http.Handler) http.Handler {
	enableRequestBodyLogging := true
	if len(logRequestBody) > 0 {
		enableRequestBodyLogging = logRequestBody[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request T
			ctx := r.Context()

			bodyBytes, err := io.ReadAll(r.Body)
			r.Body.Close()

			logger.Infow(ctx, "Raw request body read", "target_type", reflect.TypeOf(request), "body_length", len(bodyBytes))

			if enableRequestBodyLogging {
				logger.Infow(ctx, "Request body content", "body", string(bodyBytes))
			}

			if err != nil {
				logger.Errorw(ctx, "Failed to read request body", "error", err)
				httpErr := httperror.NewInternalServerError("READ_BODY_ERROR", "Failed to read request body", err, err)
				httpresponse.SendErrorJSONResponse(ctx, w, httpErr)
				return
			}

			if err := json.Unmarshal(bodyBytes, &request); err != nil {
				logger.Errorw(ctx, "Failed to decode request body", "error", err)
				httpErr := httperror.NewInternalServerError("DECODE_BODY_ERROR", "Failed to decode request body", err, err)
				httpresponse.SendErrorJSONResponse(ctx, w, httpErr)
				return
			}

			if err := validate.Struct(request); err != nil {
				logger.Infow(ctx, "Validation failed for request body", "target_type", reflect.TypeOf(request), "request", request, "error", err)
				var errors []string
				for _, err := range err.(validator.ValidationErrors) {
					message := fmt.Sprintf("%s: failed %s validation", err.Field(), err.Tag())

					if err.Param() != "" {
						message += fmt.Sprintf(" (expected: %s)", err.Param())
					}

					errors = append(errors, message)
				}

				httpErr := httperror.NewInternalServerError("VALIDATION_ERROR", "Request body validation failed", err, map[string]any{"validation_errors": errors})
				httpresponse.SendErrorJSONResponse(ctx, w, httpErr)
				return
			}

			ctx = context.WithValue(ctx, requestBodyContextKey, &request)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

const requestBodyContextKey contextKey = "requestBody"

func GetRequestBodyFromContext[T any](r *http.Request) (*T, bool) {
	body, ok := r.Context().Value(requestBodyContextKey).(*T)
	return body, ok
}
