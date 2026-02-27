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
