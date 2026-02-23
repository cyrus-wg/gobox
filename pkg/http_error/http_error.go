package httperror

import (
	"errors"
	"fmt"
	"math/rand"
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

type HTTPError struct {
	HttpStatusCode int    `json:"http_status_code"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorID        string `json:"error_id,omitempty"`
	Message        string `json:"message"`
	OriginalError  error  `json:"-"`
	ExtraInfo      any    `json:"extra_info,omitempty"`
}

func NewError(httpCode int, errorCode string, message string, originalError error, extraInfo any) *HTTPError {
	errorId := ""
	if errorIDLength > 0 {
		errorId = generateErrorID()
	}

	return &HTTPError{
		HttpStatusCode: httpCode,
		ErrorCode:      errorCode,
		ErrorID:        errorId,
		Message:        message,
		OriginalError:  originalError,
		ExtraInfo:      extraInfo,
	}
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d - ErrorCode: %s, ErrorID: %s, Message: %s, OriginalError: %v, ExtraInfo: %v",
		e.HttpStatusCode, e.ErrorCode, e.ErrorID, e.Message, e.OriginalError, e.ExtraInfo)
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

// ConvertToHTTPError attempts to convert a generic error to an HTTPError.
// It returns the HTTPError and a boolean indicating whether a new HTTPError was created (true) or if the original error was already an HTTPError (false).
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
