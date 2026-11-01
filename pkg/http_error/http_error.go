// Package httperror provides a structured HTTP error type suitable for JSON
// APIs, along with convenience constructors for common status codes and helpers
// to convert arbitrary errors into a consistent shape.
//
// All package-level configuration (error-ID length, extra-info toggle, default
// internal-server-error message/code) is stored atomically and is safe to read
// and update concurrently.
package httperror

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"sync/atomic"
)

const (
	// maxErrorIDLength is the largest permitted random error-ID length.
	maxErrorIDLength = 128

	// DefaultErrorIDLength is the default random error-ID length.
	DefaultErrorIDLength = 6
)

// Package-level configuration held in atomics for concurrency safety.
var (
	errorIDLength                     atomic.Int64
	withExtraInfo                     atomic.Bool
	defaultInternalServerErrorMessage atomic.Value // string
	defaultInternalServerErrorCode    atomic.Value // string
)

func init() {
	errorIDLength.Store(DefaultErrorIDLength)
	withExtraInfo.Store(true)
	defaultInternalServerErrorMessage.Store("Internal Server Error")
	defaultInternalServerErrorCode.Store("500")
}

// SetErrorIDLength sets the length of the random error ID generated for each
// error. A length of 0 disables ID generation. Valid range is [0, 32].
func SetErrorIDLength(length int) error {
	if length < 0 || length > maxErrorIDLength {
		return fmt.Errorf("httperror: error ID length must be between 0 and %d, got %d", maxErrorIDLength, length)
	}
	errorIDLength.Store(int64(length))
	return nil
}

// GetErrorIDLength returns the current random error-ID length.
func GetErrorIDLength() int {
	return int(errorIDLength.Load())
}

// SetWithExtraInfo toggles whether ExtraInfo is included when serialising
// errors (see http_response.SendErrorJSONResponse). Disable it in production to
// avoid leaking internal details.
func SetWithExtraInfo(enabled bool) {
	withExtraInfo.Store(enabled)
}

// IsWithExtraInfoEnabled reports whether ExtraInfo inclusion is enabled.
func IsWithExtraInfoEnabled() bool {
	return withExtraInfo.Load()
}

// SetDefaultInternalServerErrorMessage sets the fallback message used by
// NewInternalServerError when none is supplied.
func SetDefaultInternalServerErrorMessage(message string) {
	defaultInternalServerErrorMessage.Store(message)
}

// GetDefaultInternalServerErrorMessage returns the fallback ISE message.
func GetDefaultInternalServerErrorMessage() string {
	return defaultInternalServerErrorMessage.Load().(string)
}

// SetDefaultInternalServerErrorCode sets the fallback error code used by
// NewInternalServerError when none is supplied.
func SetDefaultInternalServerErrorCode(code string) {
	defaultInternalServerErrorCode.Store(code)
}

// GetDefaultInternalServerErrorCode returns the fallback ISE error code.
func GetDefaultInternalServerErrorCode() string {
	return defaultInternalServerErrorCode.Load().(string)
}

// HTTPError is a structured error that carries an HTTP status code and
// JSON-serialisable metadata. It implements error, and Unwrap exposes the
// wrapped OriginalError for use with errors.Is / errors.As.
type HTTPError struct {
	HTTPStatusCode int    `json:"http_status_code"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorID        string `json:"error_id,omitempty"`
	Message        string `json:"message"`
	OriginalError  error  `json:"-"`
	ExtraInfo      any    `json:"extra_info,omitempty"`
}

// httpErrorJSON mirrors HTTPError without a custom MarshalJSON, used inside
// MarshalJSON to avoid infinite recursion.
type httpErrorJSON struct {
	HTTPStatusCode int    `json:"http_status_code"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorID        string `json:"error_id,omitempty"`
	Message        string `json:"message"`
	ExtraInfo      any    `json:"extra_info,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for HTTPError.
// ExtraInfo is typed as `any`; certain concrete values (errors, fmt.Stringer,
// structs with no exported fields, NaN/Inf floats, maps with non-string keys,
// funcs/chans) do not serialise usefully by default. normalizeForJSON rewrites
// ExtraInfo into a JSON-friendly value first.
func (e *HTTPError) MarshalJSON() ([]byte, error) {
	return json.Marshal(httpErrorJSON{
		HTTPStatusCode: e.HTTPStatusCode,
		ErrorCode:      e.ErrorCode,
		ErrorID:        e.ErrorID,
		Message:        e.Message,
		ExtraInfo:      normalizeForJSON(e.ExtraInfo),
	})
}

// normalizeForJSON ensures the value is JSON-serialisable, converting
// otherwise-empty or invalid values to a useful string representation.
func normalizeForJSON(v any) any {
	if v == nil {
		return nil
	}

	// If it implements json.Marshaler, it knows how to serialize itself.
	if _, ok := v.(json.Marshaler); ok {
		return v
	}

	rv := reflect.ValueOf(v)
	// Dereference pointers and interfaces to get the concrete value.
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		t := rv.Type()
		for field := range t.Fields() {
			if field.IsExported() {
				return v
			}
		}
		// No exported fields → fall through to string conversion.

	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Sprintf("%v", f)
		}
		return v

	case reflect.Map:
		keyType := rv.Type().Key()
		if keyType.Kind() == reflect.String {
			return v
		}
		if keyType.Implements(textMarshalerType) {
			return v
		}
		result := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			keyStr := fmt.Sprintf("%v", iter.Key().Interface())
			result[keyStr] = normalizeForJSON(iter.Value().Interface())
		}
		return result

	case reflect.Slice, reflect.Array:
		length := rv.Len()
		result := make([]any, length)
		needsNormalization := false
		for i := range length {
			elem := rv.Index(i).Interface()
			normalized := normalizeForJSON(elem)
			result[i] = normalized
			if normalized != elem {
				needsNormalization = true
			}
		}
		if needsNormalization {
			return result
		}
		return v

	case reflect.Func, reflect.Chan:
		// Not JSON-serialisable → fall through to string conversion.

	default:
		return v
	}

	// For non-serialisable types, convert to the best string representation.
	if err, ok := v.(error); ok {
		return err.Error()
	}
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%v", v)
}

// textMarshalerType is cached for reflect type comparison in map key handling.
var textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

// New creates a new *HTTPError. A random ErrorID is attached when the
// configured error-ID length is greater than zero.
func New(httpCode int, errorCode, message string, originalError error, extraInfo any) *HTTPError {
	errorID := ""
	if n := int(errorIDLength.Load()); n > 0 {
		errorID = generateErrorID(n)
	}

	return &HTTPError{
		HTTPStatusCode: httpCode,
		ErrorCode:      errorCode,
		ErrorID:        errorID,
		Message:        message,
		OriginalError:  originalError,
		ExtraInfo:      extraInfo,
	}
}

// NewError is a backwards-compatible alias for New.
//
// Deprecated: use New.
func NewError(httpCode int, errorCode, message string, originalError error, extraInfo any) *HTTPError {
	return New(httpCode, errorCode, message, originalError, extraInfo)
}

// Error implements the error interface, producing a log-friendly string.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d - ErrorCode: %s, ErrorID: %s, Message: %s, OriginalError: %v, ExtraInfo: %v",
		e.HTTPStatusCode, e.ErrorCode, e.ErrorID, e.Message, e.OriginalError, e.ExtraInfo)
}

// Unwrap returns the wrapped OriginalError, enabling errors.Is/errors.As.
func (e *HTTPError) Unwrap() error {
	return e.OriginalError
}

// WithExtraInfo returns a shallow copy of e with ExtraInfo replaced.
func (e *HTTPError) WithExtraInfo(extraInfo any) *HTTPError {
	clone := *e
	clone.ExtraInfo = extraInfo
	return &clone
}

// WithErrorCode returns a shallow copy of e with ErrorCode replaced.
func (e *HTTPError) WithErrorCode(code string) *HTTPError {
	clone := *e
	clone.ErrorCode = code
	return &clone
}

func NewBadRequestError(errorCode, message string, originalError error, extraInfo any) *HTTPError {
	return New(400, errorCode, message, originalError, extraInfo)
}

func NewUnauthorizedError(errorCode, message string, originalError error, extraInfo any) *HTTPError {
	return New(401, errorCode, message, originalError, extraInfo)
}

func NewForbiddenError(errorCode, message string, originalError error, extraInfo any) *HTTPError {
	return New(403, errorCode, message, originalError, extraInfo)
}

func NewNotFoundError(errorCode, message string, originalError error, extraInfo any) *HTTPError {
	return New(404, errorCode, message, originalError, extraInfo)
}

func NewConflictError(errorCode, message string, originalError error, extraInfo any) *HTTPError {
	return New(409, errorCode, message, originalError, extraInfo)
}

func NewUnprocessableEntityError(errorCode, message string, originalError error, extraInfo any) *HTTPError {
	return New(422, errorCode, message, originalError, extraInfo)
}

func NewTooManyRequestsError(errorCode, message string, originalError error, extraInfo any) *HTTPError {
	return New(429, errorCode, message, originalError, extraInfo)
}

func NewServiceUnavailableError(errorCode, message string, originalError error, extraInfo any) *HTTPError {
	return New(503, errorCode, message, originalError, extraInfo)
}

func NewInternalServerError(errorCode, message string, originalError error, extraInfo any) *HTTPError {
	if message == "" {
		message = GetDefaultInternalServerErrorMessage()
	}
	if errorCode == "" {
		errorCode = GetDefaultInternalServerErrorCode()
	}
	return New(500, errorCode, message, originalError, extraInfo)
}

// As extracts an *HTTPError from err's chain, returning it and true when found.
func As(err error) (*HTTPError, bool) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr, true
	}
	return nil, false
}

// ConvertToHTTPError converts a generic error into an *HTTPError.
//
// It returns (httpErr, converted). converted is true only when err was NOT
// already an *HTTPError (in which case a wrapping 500 is created). The original
// error is preserved in OriginalError for server-side logging.
func ConvertToHTTPError(err error) (*HTTPError, bool) {
	if httpErr, ok := As(err); ok {
		return httpErr, false
	}
	return NewInternalServerError("", "", err, err), true
}

func generateErrorID(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}
