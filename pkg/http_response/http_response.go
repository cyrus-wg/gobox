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
	applicationJSON = "application/json"
	textPlain       = "text/plain"
)

// SendJSONResponse writes a JSON response with HTTP 200.
// See SendJSONResponseWithStatus for details on logResponseJson.
func SendJSONResponse(ctx context.Context, w http.ResponseWriter, payload any, logResponseJson ...bool) {
	SendJSONResponseWithStatus(ctx, w, http.StatusOK, payload, logResponseJson...)
}

// SendJSONResponseWithStatus marshals payload to JSON and writes it with the given status code.
// A nil payload is serialized as an empty JSON object {}.
//
// If marshaling fails, a 500 response is sent instead (nothing is partially written
// because we marshal to a buffer first).
//
// logResponseJson controls whether the serialized JSON is logged. It defaults to true
// for convenience, but should be set to false for responses containing sensitive data
// (e.g., tokens, credentials, PII).
func SendJSONResponseWithStatus(ctx context.Context, w http.ResponseWriter, statusCode int, payload any, logResponseJson ...bool) {
	if payload == nil {
		payload = struct{}{}
	}

	shouldLogResponseJSON := len(logResponseJson) == 0 || logResponseJson[0]

	data, err := json.Marshal(payload)
	if err != nil {
		logger.Errorw(ctx, "Failed to marshal JSON response", "marshal_error", err, "payload", payload)

		w.Header().Set(contentTypeKey, applicationJSON)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"http_status_code":500,"message":"Failed to encode response"}`))
		w.Write([]byte("\n"))
		return
	}

	if shouldLogResponseJSON {
		logger.Infow(ctx, "Sending JSON response", "http_status_code", statusCode, "response_json", string(data))
	}

	w.Header().Set(contentTypeKey, applicationJSON)
	w.WriteHeader(statusCode)
	w.Write(data)
	w.Write([]byte("\n"))
}

// SendErrorJSONResponse converts err to an *HTTPError (if not already one) and writes
// the JSON error response. Non-HTTPError errors become 500 Internal Server Error.
//
// ExtraInfo is included or omitted based on the global httperror.SetWithExtraInfo setting.
// A shallow copy of the HTTPError is made before serialization to avoid mutating the caller's object.
func SendErrorJSONResponse(ctx context.Context, w http.ResponseWriter, err error) {
	extraInfoEnabled := httperror.IsWithExtraInfoEnabled()
	httpErr, converted := httperror.ConvertToHTTPError(err)
	if converted {
		logger.Errorw(ctx, "Error response", "extra_info_enabled", extraInfoEnabled, "http_error", httpErr, "original_error", err)
	} else {
		logger.Infow(ctx, "Error response", "extra_info_enabled", extraInfoEnabled, "http_error", httpErr)
	}

	// Build a shallow copy to avoid mutating the caller's error object
	errToSend := *httpErr
	if !extraInfoEnabled {
		errToSend.ExtraInfo = nil
	}

	responseJSON, marshalErr := json.Marshal(&errToSend)
	if marshalErr != nil {
		logger.Errorw(ctx, "Failed to marshal error response", "marshal_error", marshalErr, "http_error", httpErr)
		w.Header().Set(contentTypeKey, applicationJSON)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"http_status_code":500,"message":"Internal Server Error: failed to marshal error response"}`))
		w.Write([]byte("\n"))
		return
	}

	logger.Infow(ctx, "Sending error response", "http_status_code", errToSend.HTTPStatusCode, "response_json", string(responseJSON))

	w.Header().Set(contentTypeKey, applicationJSON)
	w.WriteHeader(errToSend.HTTPStatusCode)
	w.Write(responseJSON)
	w.Write([]byte("\n"))
}

// SendPlainTextResponse writes a plain text response with HTTP 200.
func SendPlainTextResponse(ctx context.Context, w http.ResponseWriter, message string) {
	SendPlainTextResponseWithStatus(ctx, w, http.StatusOK, message)
}

// SendPlainTextResponseWithStatus writes a plain text response with the given status code.
func SendPlainTextResponseWithStatus(ctx context.Context, w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set(contentTypeKey, textPlain)
	w.WriteHeader(statusCode)
	w.Write([]byte(message))
	w.Write([]byte("\n"))
}
