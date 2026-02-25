package httpresponse

import (
	"context"
	"encoding/json"
	"net/http"

	httperror "github.com/cyrus-wg/gobox/pkg/http_error"
	"github.com/cyrus-wg/gobox/pkg/logger"
)

const (
	content_type_key = "Content-Type"
	application_json = "application/json"
	text_plain       = "text/plain"
)

func SendJSONResponse(w http.ResponseWriter, payload any) {
	w.Header().Set(content_type_key, application_json)
	if payload == nil {
		json.NewEncoder(w).Encode(struct{}{})
		return
	}
	json.NewEncoder(w).Encode(payload)
}

func SendJSONResponseWithStatus(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set(content_type_key, application_json)
	w.WriteHeader(statusCode)
	if payload == nil {
		json.NewEncoder(w).Encode(struct{}{})
		return
	}
	json.NewEncoder(w).Encode(payload)
}

func SendErrorJSONResponse(ctx context.Context, w http.ResponseWriter, err error) {
	w.Header().Set(content_type_key, application_json)

	httpErr, converted := httperror.ConvertToHTTPError(err)
	w.WriteHeader(httpErr.HttpStatusCode)

	if !httperror.IsWithExtraInfoEnabled() {
		httpErr.ExtraInfo = nil
	}

	if converted {
		logger.Errorw(ctx, "Error response", "http_error", httpErr, "error", err)
	} else {
		logger.Infow(ctx, "Error response", "http_error", httpErr)
	}

	json.NewEncoder(w).Encode(httpErr)
}

func SendPlainTextResponse(w http.ResponseWriter, message string) {
	w.Header().Set(content_type_key, text_plain)
	w.Write([]byte(message))
}

func SendPlainTextResponseWithStatus(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set(content_type_key, text_plain)
	w.WriteHeader(statusCode)
	w.Write([]byte(message))
}
