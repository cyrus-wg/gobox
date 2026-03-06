package httperror

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
)

// Test types for various ExtraInfo scenarios

type structWithExportedFields struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type structWithNoExportedFields struct {
	message string
}

func (s structWithNoExportedFields) Error() string { return s.message }

type customStringer struct {
	val string
}

func (c customStringer) String() string { return c.val }

type customMarshaler struct {
	Data string
}

func (c customMarshaler) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"custom": c.Data})
}

func TestMarshalJSON_ExtraInfo_AllTypes(t *testing.T) {
	SetErrorIDLength(0) // disable random ID for deterministic tests

	tests := []struct {
		name          string
		extraInfo     any
		wantInJSON    string // substring expected in output JSON
		wantNotInJSON string // substring that should NOT appear (optional)
	}{
		{
			name:       "nil extra info",
			extraInfo:  nil,
			wantInJSON: `"message":"test"`,
		},
		{
			name:       "string value",
			extraInfo:  "some details",
			wantInJSON: `"extra_info":"some details"`,
		},
		{
			name:       "integer value",
			extraInfo:  42,
			wantInJSON: `"extra_info":42`,
		},
		{
			name:       "float value",
			extraInfo:  3.14,
			wantInJSON: `"extra_info":3.14`,
		},
		{
			name:       "boolean value",
			extraInfo:  true,
			wantInJSON: `"extra_info":true`,
		},
		{
			name:       "map value",
			extraInfo:  map[string]string{"key": "value"},
			wantInJSON: `"extra_info":{"key":"value"}`,
		},
		{
			name:       "slice value",
			extraInfo:  []string{"a", "b", "c"},
			wantInJSON: `"extra_info":["a","b","c"]`,
		},
		{
			name:       "slice of ints",
			extraInfo:  []int{1, 2, 3},
			wantInJSON: `"extra_info":[1,2,3]`,
		},
		{
			name:       "struct with exported fields",
			extraInfo:  structWithExportedFields{Name: "test", Count: 5},
			wantInJSON: `"extra_info":{"name":"test","count":5}`,
		},
		{
			name:          "error value (no exported fields)",
			extraInfo:     errors.New("something went wrong"),
			wantInJSON:    `"extra_info":"something went wrong"`,
			wantNotInJSON: `"extra_info":{}`,
		},
		{
			name:          "fmt.Errorf error",
			extraInfo:     fmt.Errorf("wrapped: %w", errors.New("inner")),
			wantInJSON:    `"extra_info":"wrapped: inner"`,
			wantNotInJSON: `"extra_info":{}`,
		},
		{
			name:          "struct with no exported fields (implements error)",
			extraInfo:     structWithNoExportedFields{message: "custom err"},
			wantInJSON:    `"extra_info":"custom err"`,
			wantNotInJSON: `"extra_info":{}`,
		},
		{
			name:       "fmt.Stringer",
			extraInfo:  customStringer{val: "stringer output"},
			wantInJSON: `"extra_info":"stringer output"`,
		},
		{
			name:       "json.Marshaler",
			extraInfo:  customMarshaler{Data: "hello"},
			wantInJSON: `"extra_info":{"custom":"hello"}`,
		},
		{
			name:       "pointer to struct with exported fields",
			extraInfo:  &structWithExportedFields{Name: "ptr", Count: 10},
			wantInJSON: `"extra_info":{"name":"ptr","count":10}`,
		},
		{
			name:          "pointer to error",
			extraInfo:     func() any { e := structWithNoExportedFields{message: "ptr err"}; return &e }(),
			wantInJSON:    `"extra_info":"ptr err"`,
			wantNotInJSON: `"extra_info":{}`,
		},
		{
			name:       "nested map",
			extraInfo:  map[string]any{"outer": map[string]int{"inner": 1}},
			wantInJSON: `"extra_info":{"outer":{"inner":1}}`,
		},
		// --- Edge case tests ---
		{
			name:       "map with int keys",
			extraInfo:  map[int]string{1: "one", 2: "two"},
			wantInJSON: `"extra_info":{"1":"one","2":"two"}`,
		},
		{
			name:       "map with bool keys",
			extraInfo:  map[bool]string{true: "yes", false: "no"},
			wantInJSON: `"extra_info":{`,
		},
		{
			name:       "NaN float",
			extraInfo:  math.NaN(),
			wantInJSON: `"extra_info":"NaN"`,
		},
		{
			name:       "+Inf float",
			extraInfo:  math.Inf(1),
			wantInJSON: `"extra_info":"+Inf"`,
		},
		{
			name:       "-Inf float",
			extraInfo:  math.Inf(-1),
			wantInJSON: `"extra_info":"-Inf"`,
		},
		{
			name:       "normal float (not NaN/Inf)",
			extraInfo:  2.718,
			wantInJSON: `"extra_info":2.718`,
		},
		{
			name:       "slice containing errors",
			extraInfo:  []any{"ok", errors.New("bad"), 42},
			wantInJSON: `"extra_info":["ok","bad",42]`,
		},
		{
			name:          "func value",
			extraInfo:     func() {},
			wantNotInJSON: `"extra_info":{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpErr := NewBadRequestError("TEST", "test", nil, tt.extraInfo)
			data, err := json.Marshal(httpErr)
			if err != nil {
				t.Fatalf("MarshalJSON failed: %v", err)
			}

			jsonStr := string(data)

			// Always print the JSON output for visual inspection
			fmt.Printf("  %-55s → %s\n", tt.name, jsonStr)

			if tt.wantInJSON != "" {
				if !contains(jsonStr, tt.wantInJSON) {
					t.Errorf("expected JSON to contain %q, got: %s", tt.wantInJSON, jsonStr)
				}
			}
			if tt.wantNotInJSON != "" {
				if contains(jsonStr, tt.wantNotInJSON) {
					t.Errorf("expected JSON NOT to contain %q, got: %s", tt.wantNotInJSON, jsonStr)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewError
// ---------------------------------------------------------------------------

func TestNewError_BasicFields(t *testing.T) {
	SetErrorIDLength(0) // deterministic
	e := NewError(418, "TEAPOT", "I'm a teapot", nil, nil)
	if e.HTTPStatusCode != 418 {
		t.Fatalf("expected 418, got %d", e.HTTPStatusCode)
	}
	if e.ErrorCode != "TEAPOT" {
		t.Fatalf("expected TEAPOT, got %s", e.ErrorCode)
	}
	if e.Message != "I'm a teapot" {
		t.Fatalf("expected message, got %s", e.Message)
	}
	if e.ErrorID != "" {
		t.Fatalf("expected empty ErrorID when length=0, got %s", e.ErrorID)
	}
}

func TestNewError_GeneratesErrorID(t *testing.T) {
	_ = SetErrorIDLength(8)
	defer func() { _ = SetErrorIDLength(6) }()

	e := NewError(500, "X", "msg", nil, nil)
	if len(e.ErrorID) != 8 {
		t.Fatalf("expected 8-char ErrorID, got %q (len %d)", e.ErrorID, len(e.ErrorID))
	}
}

func TestNewError_PreservesOriginalError(t *testing.T) {
	orig := errors.New("boom")
	e := NewError(500, "", "", orig, nil)
	if e.OriginalError != orig {
		t.Fatal("OriginalError not preserved")
	}
}

func TestNewError_PreservesExtraInfo(t *testing.T) {
	extra := map[string]int{"count": 3}
	e := NewError(400, "", "", nil, extra)
	m, ok := e.ExtraInfo.(map[string]int)
	if !ok || m["count"] != 3 {
		t.Fatal("ExtraInfo not preserved")
	}
}

// ---------------------------------------------------------------------------
// Error() / Unwrap()
// ---------------------------------------------------------------------------

func TestHTTPError_ErrorString(t *testing.T) {
	SetErrorIDLength(0)
	e := NewError(404, "NF", "not found", errors.New("db miss"), "hint")
	s := e.Error()
	for _, want := range []string{"HTTP 404", "NF", "not found", "db miss", "hint"} {
		if !contains(s, want) {
			t.Errorf("Error() missing %q: %s", want, s)
		}
	}
}

func TestHTTPError_Unwrap(t *testing.T) {
	inner := errors.New("root cause")
	e := NewError(500, "", "", inner, nil)
	if !errors.Is(e, inner) {
		t.Fatal("Unwrap chain broken")
	}
}

func TestHTTPError_Unwrap_Nil(t *testing.T) {
	e := NewError(400, "", "", nil, nil)
	if e.Unwrap() != nil {
		t.Fatal("Unwrap should return nil when no original error")
	}
}

// ---------------------------------------------------------------------------
// Convenience constructors
// ---------------------------------------------------------------------------

func TestConvenienceConstructors(t *testing.T) {
	SetErrorIDLength(0)
	orig := errors.New("cause")

	tests := []struct {
		name     string
		fn       func(string, string, error, any) *HTTPError
		wantCode int
	}{
		{"BadRequest", NewBadRequestError, 400},
		{"Unauthorized", NewUnauthorizedError, 401},
		{"Forbidden", NewForbiddenError, 403},
		{"NotFound", NewNotFoundError, 404},
		{"Conflict", NewConflictError, 409},
		{"TooManyRequests", NewTooManyRequestsError, 429},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.fn("CODE", "msg", orig, nil)
			if e.HTTPStatusCode != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, e.HTTPStatusCode)
			}
			if e.ErrorCode != "CODE" {
				t.Errorf("expected CODE, got %s", e.ErrorCode)
			}
			if e.OriginalError != orig {
				t.Error("OriginalError not set")
			}
		})
	}
}

func TestNewInternalServerError_Defaults(t *testing.T) {
	SetErrorIDLength(0)
	SetDefaultInternalServerErrorMessage("ISE default")
	SetDefaultInternalServerErrorCode("ISE_CODE")
	defer func() {
		SetDefaultInternalServerErrorMessage("Internal Server Error")
		SetDefaultInternalServerErrorCode("500")
	}()

	e := NewInternalServerError("", "", nil, nil)
	if e.HTTPStatusCode != 500 {
		t.Fatalf("expected 500, got %d", e.HTTPStatusCode)
	}
	if e.Message != "ISE default" {
		t.Fatalf("expected default message, got %q", e.Message)
	}
	if e.ErrorCode != "ISE_CODE" {
		t.Fatalf("expected default code, got %q", e.ErrorCode)
	}
}

func TestNewInternalServerError_OverridesDefaults(t *testing.T) {
	SetErrorIDLength(0)
	e := NewInternalServerError("MY_CODE", "my message", nil, nil)
	if e.Message != "my message" {
		t.Fatalf("expected override message, got %q", e.Message)
	}
	if e.ErrorCode != "MY_CODE" {
		t.Fatalf("expected override code, got %q", e.ErrorCode)
	}
}

// ---------------------------------------------------------------------------
// ConvertToHTTPError
// ---------------------------------------------------------------------------

func TestConvertToHTTPError_AlreadyHTTPError(t *testing.T) {
	SetErrorIDLength(0)
	orig := NewBadRequestError("BR", "bad", nil, nil)
	got, converted := ConvertToHTTPError(orig)
	if converted {
		t.Fatal("should NOT be converted=true for an existing HTTPError")
	}
	if got != orig {
		t.Fatal("should return the same pointer")
	}
}

func TestConvertToHTTPError_GenericError(t *testing.T) {
	SetErrorIDLength(0)
	SetDefaultInternalServerErrorMessage("Internal Server Error")
	SetDefaultInternalServerErrorCode("500")

	plain := errors.New("something broke")
	got, converted := ConvertToHTTPError(plain)
	if !converted {
		t.Fatal("should be converted=true for a non-HTTPError")
	}
	if got.HTTPStatusCode != 500 {
		t.Fatalf("expected 500, got %d", got.HTTPStatusCode)
	}
	if got.OriginalError != plain {
		t.Fatal("OriginalError should be the plain error")
	}
}

func TestConvertToHTTPError_WrappedHTTPError(t *testing.T) {
	SetErrorIDLength(0)
	inner := NewNotFoundError("NF", "gone", nil, nil)
	wrapped := fmt.Errorf("context: %w", inner)
	got, converted := ConvertToHTTPError(wrapped)
	if converted {
		t.Fatal("errors.As should unwrap to the inner HTTPError")
	}
	if got.HTTPStatusCode != 404 {
		t.Fatalf("expected 404, got %d", got.HTTPStatusCode)
	}
}

// ---------------------------------------------------------------------------
// Config get/set
// ---------------------------------------------------------------------------

func TestSetErrorIDLength_Boundaries(t *testing.T) {
	tests := []struct {
		length  int
		wantErr bool
	}{
		{-1, true},
		{0, false},
		{1, false},
		{32, false},
		{33, true},
	}
	for _, tt := range tests {
		err := SetErrorIDLength(tt.length)
		if (err != nil) != tt.wantErr {
			t.Errorf("SetErrorIDLength(%d): got err=%v, wantErr=%v", tt.length, err, tt.wantErr)
		}
	}
	_ = SetErrorIDLength(6) // restore
}

func TestGetErrorIDLength(t *testing.T) {
	_ = SetErrorIDLength(10)
	if GetErrorIDLength() != 10 {
		t.Fatalf("expected 10, got %d", GetErrorIDLength())
	}
	_ = SetErrorIDLength(6) // restore
}

func TestWithExtraInfo_Toggle(t *testing.T) {
	SetWithExtraInfo(false)
	if IsWithExtraInfoEnabled() {
		t.Fatal("expected false")
	}
	SetWithExtraInfo(true)
	if !IsWithExtraInfoEnabled() {
		t.Fatal("expected true")
	}
}

func TestDefaultISEMessage_GetSet(t *testing.T) {
	SetDefaultInternalServerErrorMessage("custom")
	if GetDefaultInternalServerErrorMessage() != "custom" {
		t.Fatal("getter mismatch")
	}
	SetDefaultInternalServerErrorMessage("Internal Server Error") // restore
}

func TestDefaultISECode_GetSet(t *testing.T) {
	SetDefaultInternalServerErrorCode("X")
	if GetDefaultInternalServerErrorCode() != "X" {
		t.Fatal("getter mismatch")
	}
	SetDefaultInternalServerErrorCode("500") // restore
}

// ---------------------------------------------------------------------------
// generateErrorID
// ---------------------------------------------------------------------------

func TestGenerateErrorID_LengthAndCharset(t *testing.T) {
	_ = SetErrorIDLength(12)
	defer func() { _ = SetErrorIDLength(6) }()

	e := NewError(200, "", "", nil, nil)
	if len(e.ErrorID) != 12 {
		t.Fatalf("expected 12-char ID, got %q", e.ErrorID)
	}
	for _, c := range e.ErrorID {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			t.Fatalf("unexpected char %c in ErrorID", c)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
