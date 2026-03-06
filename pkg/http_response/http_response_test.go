package httpresponse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	httperror "github.com/cyrus-wg/gobox/pkg/http_error"
	"github.com/cyrus-wg/gobox/pkg/logger"
)

func init() {
	// Initialise the global logger so logger.Infow / Errorw don't panic.
	logger.InitGlobalLogger(logger.LoggerConfig{})
	// Disable error-ID randomness for deterministic tests.
	_ = httperror.SetErrorIDLength(0)
}

// ---------------------------------------------------------------------------
// SendJSONResponse / SendJSONResponseWithStatus
// ---------------------------------------------------------------------------

func TestSendJSONResponse_200(t *testing.T) {
	w := httptest.NewRecorder()
	SendJSONResponse(context.Background(), w, map[string]string{"ok": "yes"})

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
	if !strings.Contains(w.Body.String(), `"ok":"yes"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestSendJSONResponse_NilPayload(t *testing.T) {
	w := httptest.NewRecorder()
	SendJSONResponse(context.Background(), w, nil)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// nil payload → {}
	if !strings.Contains(w.Body.String(), "{}") {
		t.Fatalf("expected empty object, got: %s", w.Body.String())
	}
}

func TestSendJSONResponseWithStatus_CustomCode(t *testing.T) {
	w := httptest.NewRecorder()
	SendJSONResponseWithStatus(context.Background(), w, 201, map[string]int{"id": 42})

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"id":42`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestSendJSONResponseWithStatus_MarshalError(t *testing.T) {
	w := httptest.NewRecorder()
	// Channels cannot be marshaled to JSON.
	SendJSONResponseWithStatus(context.Background(), w, 200, make(chan int))

	if w.Code != 500 {
		t.Fatalf("expected 500 on marshal failure, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Failed to encode response") {
		t.Fatalf("expected fallback error body, got: %s", w.Body.String())
	}
}

func TestSendJSONResponseWithStatus_LogResponseFalse(t *testing.T) {
	// Just ensure it doesn't panic (logging is suppressed).
	w := httptest.NewRecorder()
	SendJSONResponseWithStatus(context.Background(), w, 200, "ok", false)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// SendErrorJSONResponse
// ---------------------------------------------------------------------------

func TestSendErrorJSONResponse_HTTPError(t *testing.T) {
	httperror.SetWithExtraInfo(true)
	defer httperror.SetWithExtraInfo(true)

	w := httptest.NewRecorder()
	httpErr := httperror.NewBadRequestError("INVALID", "bad input", nil, "extra details")
	SendErrorJSONResponse(context.Background(), w, httpErr)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"error_code":"INVALID"`) {
		t.Fatalf("expected error_code INVALID, got: %s", body)
	}
	if !strings.Contains(body, `"extra_info":"extra details"`) {
		t.Fatalf("expected extra_info, got: %s", body)
	}
}

func TestSendErrorJSONResponse_GenericError(t *testing.T) {
	httperror.SetWithExtraInfo(false)
	defer httperror.SetWithExtraInfo(true)

	w := httptest.NewRecorder()
	SendErrorJSONResponse(context.Background(), w, errors.New("something broke"))

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	body := w.Body.String()
	// Extra info should be omitted because SetWithExtraInfo(false)
	if strings.Contains(body, `"extra_info"`) {
		t.Fatalf("extra_info should be omitted, got: %s", body)
	}
}

func TestSendErrorJSONResponse_ExtraInfoDisabled(t *testing.T) {
	httperror.SetWithExtraInfo(false)
	defer httperror.SetWithExtraInfo(true)

	w := httptest.NewRecorder()
	httpErr := httperror.NewNotFoundError("NF", "gone", nil, map[string]string{"hint": "check ID"})
	SendErrorJSONResponse(context.Background(), w, httpErr)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "hint") {
		t.Fatalf("extra_info should be stripped: %s", body)
	}
}

func TestSendErrorJSONResponse_DoesNotMutateCaller(t *testing.T) {
	httperror.SetWithExtraInfo(false)
	defer httperror.SetWithExtraInfo(true)

	httpErr := httperror.NewConflictError("C", "conflict", nil, "keep me")
	w := httptest.NewRecorder()
	SendErrorJSONResponse(context.Background(), w, httpErr)

	// The caller's original error must still have ExtraInfo intact.
	if httpErr.ExtraInfo != "keep me" {
		t.Fatalf("caller's error was mutated: ExtraInfo=%v", httpErr.ExtraInfo)
	}
}

func TestSendErrorJSONResponse_ContentType(t *testing.T) {
	w := httptest.NewRecorder()
	SendErrorJSONResponse(context.Background(), w, errors.New("x"))
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

// ---------------------------------------------------------------------------
// SendPlainTextResponse / SendPlainTextResponseWithStatus
// ---------------------------------------------------------------------------

func TestSendPlainTextResponse_200(t *testing.T) {
	w := httptest.NewRecorder()
	SendPlainTextResponse(context.Background(), w, "hello")

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("expected text/plain, got %s", ct)
	}
	if !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("expected body 'hello', got %s", w.Body.String())
	}
}

func TestSendPlainTextResponseWithStatus(t *testing.T) {
	w := httptest.NewRecorder()
	SendPlainTextResponseWithStatus(context.Background(), w, 202, "accepted")

	if w.Code != 202 {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "accepted") {
		t.Fatalf("expected body 'accepted', got %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Edge: JSON response body ends with newline
// ---------------------------------------------------------------------------

func TestResponses_EndWithNewline(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
		w    *httptest.ResponseRecorder
	}{
		{
			name: "JSON",
			w:    httptest.NewRecorder(),
		},
		{
			name: "PlainText",
			w:    httptest.NewRecorder(),
		},
		{
			name: "Error",
			w:    httptest.NewRecorder(),
		},
	}

	SendJSONResponse(context.Background(), tests[0].w, "ok")
	SendPlainTextResponse(context.Background(), tests[1].w, "ok")
	SendErrorJSONResponse(context.Background(), tests[2].w, errors.New("e"))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := tt.w.Body.String()
			if !strings.HasSuffix(body, "\n") {
				t.Fatalf("expected trailing newline, got %q", body)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Edge: verify JSON is valid on success path
// ---------------------------------------------------------------------------

func TestSendJSONResponse_ValidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	SendJSONResponse(context.Background(), w, map[string][]int{"nums": {1, 2, 3}})

	var out map[string][]int
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(out["nums"]) != 3 {
		t.Fatalf("expected 3 nums, got %v", out["nums"])
	}
}
