package httperror

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
)

var (
	errorIDLength                     = 6
	withExtraInfo                     = true
	defaultInternalServerErrorMessage = "Internal Server Error"
	defaultInternalServerErrorCode    = "500"
)

func SetErrorIDLength(length int) error {
	if (length < 0) || (length > 32) {
		return errors.New("error ID length must be between 0 and 32")
	}

	errorIDLength = length
	return nil
}

func GetErrorIDLength() int {
	return errorIDLength
}

func SetWithExtraInfo(enabled bool) {
	withExtraInfo = enabled
}

func IsWithExtraInfoEnabled() bool {
	return withExtraInfo
}

func SetDefaultInternalServerErrorMessage(message string) {
	defaultInternalServerErrorMessage = message
}

func GetDefaultInternalServerErrorMessage() string {
	return defaultInternalServerErrorMessage
}

func SetDefaultInternalServerErrorCode(code string) {
	defaultInternalServerErrorCode = code
}

func GetDefaultInternalServerErrorCode() string {
	return defaultInternalServerErrorCode
}

type HTTPError struct {
	HTTPStatusCode int    `json:"http_status_code"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorID        string `json:"error_id,omitempty"`
	Message        string `json:"message"`
	OriginalError  error  `json:"-"`
	ExtraInfo      any    `json:"extra_info,omitempty"`
}

// httpErrorJSON is an alias used inside MarshalJSON to avoid infinite recursion.
type httpErrorJSON struct {
	HTTPStatusCode int    `json:"http_status_code"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorID        string `json:"error_id,omitempty"`
	Message        string `json:"message"`
	ExtraInfo      any    `json:"extra_info,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for HTTPError.
// This is needed because ExtraInfo is typed as `any`, and certain concrete types
// stored in the interface (e.g., error, fmt.Stringer, or structs with no exported
// fields) have no exported fields visible to encoding/json, causing it to produce
// "{}" instead of a meaningful value. This method normalizes ExtraInfo before
// serialization to ensure all types produce useful JSON output.
func (e *HTTPError) MarshalJSON() ([]byte, error) {
	return json.Marshal(httpErrorJSON{
		HTTPStatusCode: e.HTTPStatusCode,
		ErrorCode:      e.ErrorCode,
		ErrorID:        e.ErrorID,
		Message:        e.Message,
		ExtraInfo:      normalizeForJSON(e.ExtraInfo),
	})
}

// normalizeForJSON ensures that the value is properly JSON-serializable.
//
// It handles the following cases:
//   - nil: returned as-is
//   - json.Marshaler: returned as-is (it knows how to serialize itself)
//   - Struct with exported fields: returned as-is
//   - Struct with NO exported fields: converted to string via error.Error(),
//     fmt.Stringer.String(), or fmt.Sprintf as fallback
//   - Float64/Float32 NaN or ±Inf: converted to string (JSON spec forbids these)
//   - Map with non-string keys: converted to map[string]any with keys stringified
//   - Primitives (string, int, float, bool), slices, arrays: returned as-is
//   - Non-serializable types (func, chan): converted to string representation
func normalizeForJSON(v any) any {
	if v == nil {
		return nil
	}

	// If it implements json.Marshaler, it knows how to serialize itself
	if _, ok := v.(json.Marshaler); ok {
		return v
	}

	rv := reflect.ValueOf(v)
	// Dereference pointers and interfaces to get the concrete value
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		// Check if struct has any exported fields that json.Marshal can see
		t := rv.Type()
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).IsExported() {
				// Has exported fields - json.Marshal will produce meaningful output
				return v
			}
		}
		// No exported fields → fall through to string conversion

	case reflect.Float32, reflect.Float64:
		// JSON spec does not allow NaN or ±Inf; json.Marshal would return an error
		f := rv.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Sprintf("%v", f)
		}
		return v

	case reflect.Map:
		// JSON requires string keys. If the map key type is string, return as-is.
		// If the key implements encoding.TextMarshaler, json.Marshal handles it.
		// Otherwise, convert keys to strings to avoid a marshal error.
		keyType := rv.Type().Key()
		if keyType.Kind() == reflect.String {
			return v
		}
		if keyType.Implements(textMarshalerType) {
			return v
		}
		// Convert map[K]V → map[string]any with keys stringified
		result := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			keyStr := fmt.Sprintf("%v", iter.Key().Interface())
			result[keyStr] = normalizeForJSON(iter.Value().Interface())
		}
		return result

	case reflect.Slice, reflect.Array:
		// Recursively normalize slice/array elements (they may contain non-serializable values)
		length := rv.Len()
		result := make([]any, length)
		needsNormalization := false
		for i := 0; i < length; i++ {
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
		// These types are not JSON-serializable → fall through to string conversion

	default:
		// Primitives (string, int, bool) all serialize correctly
		return v
	}

	// For non-serializable types, convert to the best string representation available
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

func NewError(httpCode int, errorCode string, message string, originalError error, extraInfo any) *HTTPError {
	errorId := ""
	if errorIDLength > 0 {
		errorId = generateErrorID()
	}

	return &HTTPError{
		HTTPStatusCode: httpCode,
		ErrorCode:      errorCode,
		ErrorID:        errorId,
		Message:        message,
		OriginalError:  originalError,
		ExtraInfo:      extraInfo,
	}
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d - ErrorCode: %s, ErrorID: %s, Message: %s, OriginalError: %v, ExtraInfo: %v",
		e.HTTPStatusCode, e.ErrorCode, e.ErrorID, e.Message, e.OriginalError, e.ExtraInfo)
}

func (e *HTTPError) Unwrap() error {
	return e.OriginalError
}

func NewBadRequestError(errorCode string, message string, originalError error, extraInfo any) *HTTPError {
	return NewError(400, errorCode, message, originalError, extraInfo)
}

func NewUnauthorizedError(errorCode string, message string, originalError error, extraInfo any) *HTTPError {
	return NewError(401, errorCode, message, originalError, extraInfo)
}

func NewForbiddenError(errorCode string, message string, originalError error, extraInfo any) *HTTPError {
	return NewError(403, errorCode, message, originalError, extraInfo)
}

func NewNotFoundError(errorCode string, message string, originalError error, extraInfo any) *HTTPError {
	return NewError(404, errorCode, message, originalError, extraInfo)
}

func NewConflictError(errorCode string, message string, originalError error, extraInfo any) *HTTPError {
	return NewError(409, errorCode, message, originalError, extraInfo)
}

func NewTooManyRequestsError(errorCode string, message string, originalError error, extraInfo any) *HTTPError {
	return NewError(429, errorCode, message, originalError, extraInfo)
}

func NewInternalServerError(errorCode string, message string, originalError error, extraInfo any) *HTTPError {
	if message == "" {
		message = defaultInternalServerErrorMessage
	}
	if errorCode == "" {
		errorCode = defaultInternalServerErrorCode
	}
	return NewError(500, errorCode, message, originalError, extraInfo)
}

// ConvertToHTTPError attempts to convert a generic error to an *HTTPError.
// Returns (httpErr, converted) where converted=true means the original error was NOT an *HTTPError and a new one was created to wrap it.
// The original error is preserved in OriginalError for server-side logging, but is not included in the JSON response.
func ConvertToHTTPError(err error) (*HTTPError, bool) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr, false
	}

	return NewInternalServerError("", "", err, err), true
}

func generateErrorID() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, errorIDLength)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
