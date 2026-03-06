package pattern

import "testing"

// ─── MatchAnt ───────────────────────────────────────────────────────────────

func TestMatchAnt(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		// Exact match
		{"/api/users", "/api/users", true},
		{"/", "/", true},

		// Single-segment wildcard (*)
		{"/api/*", "/api/users", true},
		{"/api/*", "/api/orders", true},
		{"/api/*", "/api/users/123", false}, // * only matches one segment

		// Multi-segment wildcard (**)
		{"/api/**", "/api/v1/users", true},
		{"/api/**", "/api/v1/users/123", true},
		{"/api/**", "/api", true}, // ** matches zero segments

		// Single-char wildcard (?)
		{"/user/?", "/user/a", true},
		{"/user/?", "/user/ab", false}, // ? matches exactly one char

		// Combined patterns
		{"config/*/host", "config/db/host", true},
		{"config/*/host", "config/db/port", false},

		// ** in the middle
		{"/api/**/detail", "/api/v1/users/detail", true},
		{"/api/**/detail", "/api/detail", true},

		// ** as suffix match
		{"**.log", "app.service.log", true},
		{"**.log", "just.log", true},
		{"**.log", "logfile", false},

		// Leading/trailing slash trimming
		{"api/*", "api/users", true},
		{"/api/*", "api/users", true},
		{"api/*", "/api/users", true},

		// No match
		{"/api/users", "/api/orders", false},
		{"/api/*", "/other/path", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.input, func(t *testing.T) {
			got := MatchAnt(tt.pattern, tt.input)
			if got != tt.want {
				t.Errorf("MatchAnt(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

// ─── MatchWildcard ──────────────────────────────────────────────────────────

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		// Exact
		{"hello", "hello", true},
		{"hello", "world", false},

		// ? matches one char
		{"f?o", "foo", true},
		{"f?o", "fao", true},
		{"f?o", "fo", false},
		{"f?o", "fooo", false},

		// * matches zero or more (including /)
		{"foo*", "foobar", true},
		{"foo*", "foo", true},
		{"*.json", "data.json", true},
		{"*.json", "a/b.json", true}, // * crosses /
		{"*", "", true},
		{"*", "anything", true},

		// Combined
		{"h?llo*", "hello world", true},
		{"h?llo*", "hallo!", true},

		// Empty
		{"", "", true},
		{"", "x", false},
		{"?", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.input, func(t *testing.T) {
			got := MatchWildcard(tt.pattern, tt.input)
			if got != tt.want {
				t.Errorf("MatchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

// ─── MatchRegex ─────────────────────────────────────────────────────────────

func TestMatchRegex(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		// Basic
		{"/api/users", "/api/users", true},
		{"/api/users", "/api/users/extra", false}, // auto-anchored

		// Regex features
		{`/api/users/\d+`, "/api/users/123", true},
		{`/api/users/\d+`, "/api/users/abc", false},
		{`/api/(v1|v2)/.*`, "/api/v1/anything", true},
		{`/api/(v1|v2)/.*`, "/api/v3/anything", false},

		// Invalid regex → false
		{"[invalid", "/something", false},

		// Empty
		{"", "", true},
		{".*", "anything at all", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.input, func(t *testing.T) {
			got := MatchRegex(tt.pattern, tt.input)
			if got != tt.want {
				t.Errorf("MatchRegex(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}
