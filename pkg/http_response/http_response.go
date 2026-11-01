// Package httpresponse provides small helpers for writing JSON, plain-text, and
// structured error responses from net/http handlers. Error responses reuse the
// httperror package so status codes and payload shapes stay consistent across a
// service.
package httpresponse

import (
	"context"
	"encoding/json"
	"net/http"

	httperror "github.com/cyrus-wg/gobox/pkg/http_error"
	"github.com/cyrus-wg/gobox/pkg/logger"
)

const (
	contentTypeKey  = "Content-Type"
	applicationJSON = "application/json; charset=utf-8"
	textPlain       = "text/plain; charset=utf-8"
)

// SendJSONResponse writes payload as JSON with HTTP 200.
// See SendJSONResponseWithStatus for the meaning of logResponseJSON.
func SendJSONResponse(ctx context.Context, w http.ResponseWriter, payload any, logResponseJSON ...bool) {
	SendJSONResponseWithStatus(ctx, w, http.StatusOK, payload, logResponseJSON...)
}

// SendJSONResponseWithStatus marshals payload to JSON and writes it with the
// given status code. A nil payload is serialised as an empty JSON object {}.
//
// If marshaling fails, a 500 response is sent instead — nothing is partially
// written because the payload is marshaled to a buffer first.
//
// logResponseJSON controls whether the serialised JSON is logged (default
// true). Set it to false for responses containing sensitive data such as
// tokens, credentials, or PII.
func SendJSONResponseWithStatus(ctx context.Context, w http.ResponseWriter, statusCode int, payload any, logResponseJSON ...bool) {
	if payload == nil {
		payload = struct{}{}
	}

	shouldLog := len(logResponseJSON) == 0 || logResponseJSON[0]

	data, err := json.Marshal(payload)
	if err != nil {
		logger.Errorw(ctx, "Failed to marshal JSON response", "marshal_error", err)

		w.Header().Set(contentTypeKey, applicationJSON)
		w.WriteHeader(http.StatusInternalServerError)
		writeLine(w, []byte(`{"http_status_code":500,"message":"Failed to encode response"}`))
		return
	}

	if shouldLog {
		logger.Infow(ctx, "Sending JSON response", "http_status_code", statusCode, "response_json", string(data))
	} else {
		logger.Infow(ctx, "Sending JSON response", "http_status_code", statusCode, "response_json", "<omitted>")
	}

	w.Header().Set(contentTypeKey, applicationJSON)
	w.WriteHeader(statusCode)
	writeLine(w, data)
}

// SendNoContent writes a 204 No Content response with no body.
func SendNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// SendErrorJSONResponse converts err to an *HTTPError (if not already one) and
// writes the JSON error response. Non-HTTPError errors become 500.
//
// ExtraInfo is included or omitted based on httperror.SetWithExtraInfo. A
// shallow copy of the HTTPError is serialised so the caller's object is never
// mutated.
func SendErrorJSONResponse(ctx context.Context, w http.ResponseWriter, err error) {
	extraInfoEnabled := httperror.IsWithExtraInfoEnabled()
	httpErr, converted := httperror.ConvertToHTTPError(err)
	if converted {
		logger.Errorw(ctx, "Error response", "extra_info_enabled", extraInfoEnabled, "http_error", httpErr, "original_error", err)
	} else {
		logger.Infow(ctx, "Error response", "extra_info_enabled", extraInfoEnabled, "http_error", httpErr)
	}

	// Build a shallow copy to avoid mutating the caller's error object.
	errToSend := *httpErr
	if !extraInfoEnabled {
		errToSend.ExtraInfo = nil
	}

	responseJSON, marshalErr := json.Marshal(&errToSend)
	if marshalErr != nil {
		logger.Errorw(ctx, "Failed to marshal error response", "marshal_error", marshalErr, "http_error", httpErr)
		w.Header().Set(contentTypeKey, applicationJSON)
		w.WriteHeader(http.StatusInternalServerError)
		writeLine(w, []byte(`{"http_status_code":500,"message":"Internal Server Error: failed to marshal error response"}`))
		return
	}

	logger.Infow(ctx, "Sending error response", "http_status_code", errToSend.HTTPStatusCode, "response_json", string(responseJSON))

	w.Header().Set(contentTypeKey, applicationJSON)
	w.WriteHeader(errToSend.HTTPStatusCode)
	writeLine(w, responseJSON)
}

// SendPlainTextResponse writes a plain-text response with HTTP 200.
func SendPlainTextResponse(ctx context.Context, w http.ResponseWriter, message string) {
	SendPlainTextResponseWithStatus(ctx, w, http.StatusOK, message)
}

// SendPlainTextResponseWithStatus writes a plain-text response with the given
// status code.
func SendPlainTextResponseWithStatus(_ context.Context, w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set(contentTypeKey, textPlain)
	w.WriteHeader(statusCode)
	writeLine(w, []byte(message))
}

// writeLine writes data followed by a trailing newline. Write errors are
// ignored because the header/status has already been committed and there is no
// meaningful recovery for a broken connection here.
func writeLine(w http.ResponseWriter, data []byte) {
	_, _ = w.Write(data)
	_, _ = w.Write([]byte("\n"))
}
