// Package pattern provides three string-matching strategies commonly needed
// for routing, log filtering, and namespace matching:
//
//   - MatchAnt: Ant-style globs with "/" segment semantics (*, **, ?).
//   - MatchWildcard: flat shell-style globs (*, ?) with no segment semantics.
//   - MatchRegex: anchored regular expressions with an internal compile cache.
package pattern

import (
	"regexp"
	"strings"
	"sync"
)

// MatchAnt reports whether s matches the given Ant-style glob pattern.
//
// The pattern uses "/" as a segment separator. Wildcards:
//
//   - ?  matches exactly one character within a single segment
//   - *  matches zero or more characters within a single segment
//   - ** matches zero or more complete segments (including none)
//
// Common uses include URL paths, file paths, config key hierarchies, and any
// dot- or slash-delimited namespace (e.g. "org.**.service.*").
//
// Examples:
//
//	MatchAnt("/api/*",        "/api/users")       // true  – single segment wildcard
//	MatchAnt("/api/**",       "/api/v1/users")    // true  – multi-segment wildcard
//	MatchAnt("/user/?",       "/user/a")          // true  – single char wildcard
//	MatchAnt("/user/?",       "/user/ab")         // false – ? matches exactly one char
//	MatchAnt("**.log",        "app.service.log")  // true  – suffix match across segments
//	MatchAnt("config/*/host", "config/db/host")   // true  – middle segment wildcard
func MatchAnt(pattern, s string) bool {
	if pattern == s {
		return true
	}
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	sParts := strings.Split(strings.Trim(s, "/"), "/")
	return matchAntParts(patternParts, sParts)
}

// MatchWildcard reports whether s matches pattern using ? and * wildcards.
//
// Unlike MatchAnt, this operates on a flat string with no segment semantics —
// * can match across any character including "/" or ".".
//
//   - ? matches exactly one character
//   - * matches zero or more characters
//
// Examples:
//
//	MatchWildcard("foo*",   "foobar")    // true
//	MatchWildcard("f?o",    "foo")       // true
//	MatchWildcard("*.json", "data.json") // true
//	MatchWildcard("*.json", "a/b.json")  // true  – * crosses "/"
func MatchWildcard(pattern, s string) bool {
	return matchWildcard(pattern, s)
}

// MatchRegex reports whether path matches the given regular expression.
// The pattern is automatically anchored (^ and $) so partial matches are
// rejected. Compiled patterns are cached, so repeated calls with the same
// pattern avoid recompilation. Returns false if the pattern fails to compile.
func MatchRegex(regexPattern, path string) bool {
	re, err := compileAnchored(regexPattern)
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

// Compile returns an anchored, cached *regexp.Regexp for the given pattern.
// It is useful when a caller wants to compile once and reuse, or to surface a
// compilation error explicitly (unlike MatchRegex, which swallows it).
func Compile(regexPattern string) (*regexp.Regexp, error) {
	return compileAnchored(regexPattern)
}

// regexCache memoises anchored regexp compilations keyed by the raw pattern.
var regexCache sync.Map // map[string]*regexpCacheEntry

type regexpCacheEntry struct {
	re  *regexp.Regexp
	err error
}

func compileAnchored(pattern string) (*regexp.Regexp, error) {
	if v, ok := regexCache.Load(pattern); ok {
		entry := v.(*regexpCacheEntry)
		return entry.re, entry.err
	}
	re, err := regexp.Compile("^" + pattern + "$")
	// LoadOrStore keeps the first successful/observed compilation and avoids
	// storing duplicates under races.
	actual, _ := regexCache.LoadOrStore(pattern, &regexpCacheEntry{re: re, err: err})
	entry := actual.(*regexpCacheEntry)
	return entry.re, entry.err
}

func matchAntParts(patternParts, pathParts []string) bool {
	patternIdx, pathIdx := 0, 0

	for patternIdx < len(patternParts) && pathIdx < len(pathParts) {
		patternPart := patternParts[patternIdx]

		if patternPart == "**" {
			if patternIdx == len(patternParts)-1 {
				return true
			}
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

	for patternIdx < len(patternParts) && patternParts[patternIdx] == "**" {
		patternIdx++
	}

	return patternIdx == len(patternParts) && pathIdx == len(pathParts)
}

func matchSegment(pattern, segment string) bool {
	if pattern == "*" {
		return true
	}
	return matchWildcard(pattern, segment)
}

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
