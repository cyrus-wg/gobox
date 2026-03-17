package middleware

import (
	"context"
	"encoding/json"
	"errors"
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
				httpErr := httperror.NewBadRequestError("DECODE_BODY_ERROR", "Failed to decode request body", err, err)
				httpresponse.SendErrorJSONResponse(ctx, w, httpErr)
				return
			}

			if err := validate.Struct(request); err != nil {
				logger.Infow(ctx, "Validation failed for request body", "target_type", reflect.TypeOf(request), "request", request, "error", err)

				var validationErrs validator.ValidationErrors
				if !errors.As(err, &validationErrs) {
					// Not a ValidationErrors (e.g., InvalidValidationError) — treat as internal error
					httpErr := httperror.NewInternalServerError("REQUEST_BODY_VALIDATION_ERROR", "Failed to process request body validation", err, err)
					httpresponse.SendErrorJSONResponse(ctx, w, httpErr)
					return
				}

				var fieldErrors []string
				for _, fe := range validationErrs {
					message := fmt.Sprintf("%s: failed %s validation", fe.Field(), fe.Tag())
					if fe.Param() != "" {
						message += fmt.Sprintf(" (expected: %s)", fe.Param())
					}
					fieldErrors = append(fieldErrors, message)
				}

				httpErr := httperror.NewBadRequestError("REQUEST_BODY_CONSTRAINT_VIOLATION", "Request body violates constraints", err, map[string]any{"validation_errors": fieldErrors})
				httpresponse.SendErrorJSONResponse(ctx, w, httpErr)
				return
			}

			ctx = context.WithValue(ctx, requestBodyContextKey, &request)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

const requestBodyContextKey contextKey = "requestBody"

func GetRequestBodyFromContext[T any](ctx context.Context) (*T, bool) {
	body, ok := ctx.Value(requestBodyContextKey).(*T)
	return body, ok
}
