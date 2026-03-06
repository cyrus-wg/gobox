package main

import (
	"fmt"

	"github.com/cyrus-wg/gobox/pkg/pattern"
)

func main() {
	fmt.Println("=== pattern package examples ===")
	fmt.Println()

	// --- Ant-style glob matching ---
	fmt.Println("// MatchAnt — Ant-style glob patterns")
	examples := []struct {
		pat, str string
	}{
		{"/api/v1/**", "/api/v1/users/123"},
		{"/api/v1/**", "/api/v2/users"},
		{"/static/*.css", "/static/main.css"},
		{"/static/*.css", "/static/deep/nested.css"},
		{"/health", "/health"},
		{"/user/?", "/user/A"},
	}
	for _, e := range examples {
		fmt.Printf("  MatchAnt(%q, %q) → %v\n", e.pat, e.str, pattern.MatchAnt(e.pat, e.str))
	}
	fmt.Println()

	// --- Wildcard matching ---
	fmt.Println("// MatchWildcard — simple * and ? wildcards")
	wcExamples := []struct {
		pat, str string
	}{
		{"hello*", "hello world"},
		{"h?llo", "hello"},
		{"*.json", "config.json"},
	}
	for _, e := range wcExamples {
		fmt.Printf("  MatchWildcard(%q, %q) → %v\n", e.pat, e.str, pattern.MatchWildcard(e.pat, e.str))
	}
	fmt.Println()

	// --- Regex matching ---
	fmt.Println("// MatchRegex — full regex")
	rxExamples := []struct {
		pat, str string
	}{
		{`/api/users/\d+`, "/api/users/123"},
		{`/api/users/\d+`, "/api/users/abc"},
		{`/api/(v1|v2)/.*`, "/api/v1/anything"},
	}
	for _, e := range rxExamples {
		fmt.Printf("  MatchRegex(%q, %q) → %v\n", e.pat, e.str, pattern.MatchRegex(e.pat, e.str))
	}
}
