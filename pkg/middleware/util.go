package middleware

import (
	"net"
	"net/http"
	"regexp"
	"strings"
)

func getRealUserIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	if xci := r.Header.Get("X-Client-IP"); xci != "" {
		return xci
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

type BypassRequestLogging struct {
	Path    string
	Methods string         // Comma-separated methods (e.g., "GET,POST"), empty means all methods
	IsRegex bool           // If true, Path is treated as a regex pattern; otherwise Ant-style pattern
	regex   *regexp.Regexp // Pre-compiled regex (internal use)
}

// compileBypassPatterns pre-compiles regex patterns automatically.
// This is called internally when middleware is initialized.
func compileBypassPatterns(patterns []BypassRequestLogging) []BypassRequestLogging {
	compiled := make([]BypassRequestLogging, len(patterns))
	for i, p := range patterns {
		compiled[i] = p
		if p.IsRegex {
			if re, err := regexp.Compile("^" + p.Path + "$"); err == nil {
				compiled[i].regex = re
			}
		}
	}
	return compiled
}

func shouldBypassMiddlewareLogging(bypassList []BypassRequestLogging, path string, method string) bool {
	for _, bypass := range bypassList {
		if !matchMethod(bypass.Methods, method) {
			continue
		}

		if bypass.IsRegex {
			if matchRegex(&bypass, path) {
				return true
			}
		} else {
			if matchAntPattern(bypass.Path, path) {
				return true
			}
		}
	}

	return false
}

// matchMethod checks if the request method matches the allowed methods.
// Empty methods string means all methods are allowed.
func matchMethod(methods string, method string) bool {
	if methods == "" {
		return true
	}

	// Fast path for single method (no comma)
	if !strings.Contains(methods, ",") {
		return strings.EqualFold(strings.TrimSpace(methods), method)
	}

	// Split and check each method
	for m := range strings.SplitSeq(methods, ",") {
		if strings.EqualFold(strings.TrimSpace(m), method) {
			return true
		}
	}
	return false
}

// matchAntPattern matches path against Ant-style pattern (Spring Security style)
// Supports:
//   - ? matches one character
//   - * matches zero or more characters within a path segment
//   - ** matches zero or more path segments
func matchAntPattern(pattern, path string) bool {
	// Exact match
	if pattern == path {
		return true
	}

	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	return matchAntParts(patternParts, pathParts)
}

func matchAntParts(patternParts, pathParts []string) bool {
	patternIdx, pathIdx := 0, 0

	for patternIdx < len(patternParts) && pathIdx < len(pathParts) {
		patternPart := patternParts[patternIdx]

		if patternPart == "**" {
			// ** at the end matches everything
			if patternIdx == len(patternParts)-1 {
				return true
			}

			// Try to match ** with varying number of path segments
			for i := pathIdx; i <= len(pathParts); i++ {
				if matchAntParts(patternParts[patternIdx+1:], pathParts[i:]) {
					return true
				}
			}
			return false
		}

		if !matchSegment(patternPart, pathParts[pathIdx]) {
			return false
		}

		patternIdx++
		pathIdx++
	}

	// Handle trailing ** in pattern
	for patternIdx < len(patternParts) && patternParts[patternIdx] == "**" {
		patternIdx++
	}

	return patternIdx == len(patternParts) && pathIdx == len(pathParts)
}

// matchSegment matches a single path segment against a pattern segment
// Supports ? (one char) and * (zero or more chars within segment)
func matchSegment(pattern, segment string) bool {
	if pattern == "*" {
		return true
	}

	return matchWildcard(pattern, segment)
}

// matchWildcard matches string against pattern with ? and * wildcards
func matchWildcard(pattern, str string) bool {
	pLen, sLen := len(pattern), len(str)
	pIdx, sIdx := 0, 0
	starIdx, matchIdx := -1, 0

	for sIdx < sLen {
		if pIdx < pLen && (pattern[pIdx] == '?' || pattern[pIdx] == str[sIdx]) {
			pIdx++
			sIdx++
		} else if pIdx < pLen && pattern[pIdx] == '*' {
			starIdx = pIdx
			matchIdx = sIdx
			pIdx++
		} else if starIdx != -1 {
			pIdx = starIdx + 1
			matchIdx++
			sIdx = matchIdx
		} else {
			return false
		}
	}

	for pIdx < pLen && pattern[pIdx] == '*' {
		pIdx++
	}

	return pIdx == pLen
}

// matchRegex matches path against a regex pattern
// Uses pre-compiled regex if available, otherwise compiles on the fly
func matchRegex(bypass *BypassRequestLogging, path string) bool {
	// Use pre-compiled regex if available
	if bypass.regex != nil {
		return bypass.regex.MatchString(path)
	}

	// Fallback: compile and match (less efficient)
	matched, err := regexp.MatchString("^"+bypass.Path+"$", path)
	if err != nil {
		return false
	}
	return matched
}
