package pick

import "strconv"

// ---------------------------------------------------------------------------
// Conditional selection
// ---------------------------------------------------------------------------

// If returns trueVal when condition is true, otherwise falseVal.
// It is the equivalent of the ternary operator (condition ? trueVal : falseVal)
// absent in Go.
//
// Both arguments are eagerly evaluated before the call (standard Go behaviour).
// Use IfFunc when either branch is expensive or has side-effects.
//
//	label := pick.If(count == 1, "item", "items")
//	status := pick.If(err != nil, "error", "ok")
func If[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}

// IfFunc is the lazy variant of If. trueFn is only called when condition is
// true; falseFn is only called when condition is false. Use this when
// producing either value is expensive or has side-effects.
//
//	token := pick.IfFunc(cached, func() string { return cache.Get() },
//	                             func() string { return api.FetchToken() })
func IfFunc[T any](condition bool, trueFn, falseFn func() T) T {
	if condition {
		return trueFn()
	}
	return falseFn()
}

// ---------------------------------------------------------------------------
// Generic helpers
//
// All functions follow the standard Go (value, ok) convention:
//   ok = true  → the original/found value is returned
//   ok = false → defaultValue was used as a fallback
// ---------------------------------------------------------------------------

// OrDefault returns value with ok=true when value is not the zero value of T.
// If value IS the zero value, defaultValue is returned with ok=false.
//
//	name, _  := pick.OrDefault(req.Name, "anonymous")
//	count, ok := pick.OrDefault(cfg.MaxRetries, 3)
func OrDefault[T comparable](value T, defaultValue T) (T, bool) {
	var zero T
	if value == zero {
		return defaultValue, false
	}
	return value, true
}

// OrDefaultPtr dereferences ptr and returns the pointee with ok=true when ptr
// is non-nil. If ptr is nil, defaultValue is returned with ok=false.
// Use this for optional pointer fields common in JSON / protobuf / query params.
//
//	timeout, _ := pick.OrDefaultPtr(req.TimeoutSeconds, 30)
func OrDefaultPtr[T any](ptr *T, defaultValue T) (T, bool) {
	if ptr == nil {
		return defaultValue, false
	}
	return *ptr, true
}

// OrDefaultFunc returns value with ok=true when value is not the zero value of T.
// If value IS the zero value, fn is called and its result is returned with ok=false.
// Prefer this over OrDefault when producing the default is expensive or has
// side-effects that should only happen when actually needed.
//
//	dsn, _ := pick.OrDefaultFunc(cfg.DSN, loadDSNFromSecretManager)
func OrDefaultFunc[T comparable](value T, fn func() T) (T, bool) {
	var zero T
	if value == zero {
		return fn(), false
	}
	return value, true
}

// Coalesce returns the first argument that is not the zero value of T with
// ok=true, mirroring the SQL COALESCE function. If all arguments are zero,
// the zero value of T is returned with ok=false.
//
//	host, _ := pick.Coalesce(os.Getenv("HOST"), cfg.Host, "localhost")
func Coalesce[T comparable](values ...T) (T, bool) {
	var zero T
	for _, v := range values {
		if v != zero {
			return v, true
		}
	}
	return zero, false
}

// MapOrDefault looks up key in m and returns the associated value with ok=true.
// If m is nil or key is not present, defaultValue is returned with ok=false.
//
//	role, _ := pick.MapOrDefault(userRoles, userID, "guest")
func MapOrDefault[K comparable, V any](m map[K]V, key K, defaultValue V) (V, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	return defaultValue, false
}

// SliceFirstOrDefault returns the first element of slice with ok=true.
// If slice is nil or empty, defaultValue is returned with ok=false.
//
//	primary, _ := pick.SliceFirstOrDefault(addresses, fallbackAddr)
func SliceFirstOrDefault[T any](slice []T, defaultValue T) (T, bool) {
	if len(slice) == 0 {
		return defaultValue, false
	}
	return slice[0], true
}

// ---------------------------------------------------------------------------
// String-to-type parsers
//
// Each function parses str into the target type.
//   ok = true  → parsing succeeded; the parsed value is returned
//   ok = false → parsing failed; defaultValue is returned
// ---------------------------------------------------------------------------

// StringOrDefault returns str with ok=true when str is non-empty.
// If str is empty, defaultValue is returned with ok=false.
func StringOrDefault(str string, defaultValue string) (string, bool) {
	if str == "" {
		return defaultValue, false
	}
	return str, true
}

// IntOrDefault parses str as a base-10 int.
func IntOrDefault(str string, defaultValue int) (int, bool) {
	value, err := strconv.Atoi(str)
	if err != nil {
		return defaultValue, false
	}
	return value, true
}

// Int32OrDefault parses str as a base-10 int32.
func Int32OrDefault(str string, defaultValue int32) (int32, bool) {
	value, err := strconv.ParseInt(str, 10, 32)
	if err != nil {
		return defaultValue, false
	}
	return int32(value), true
}

// Int64OrDefault parses str as a base-10 int64.
func Int64OrDefault(str string, defaultValue int64) (int64, bool) {
	value, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return defaultValue, false
	}
	return value, true
}

// UintOrDefault parses str as a base-10 uint.
func UintOrDefault(str string, defaultValue uint) (uint, bool) {
	value, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return defaultValue, false
	}
	return uint(value), true
}

// Uint32OrDefault parses str as a base-10 uint32.
func Uint32OrDefault(str string, defaultValue uint32) (uint32, bool) {
	value, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return defaultValue, false
	}
	return uint32(value), true
}

// Uint64OrDefault parses str as a base-10 uint64.
func Uint64OrDefault(str string, defaultValue uint64) (uint64, bool) {
	value, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return defaultValue, false
	}
	return value, true
}

// Float32OrDefault parses str as a float32.
func Float32OrDefault(str string, defaultValue float32) (float32, bool) {
	value, err := strconv.ParseFloat(str, 32)
	if err != nil {
		return defaultValue, false
	}
	return float32(value), true
}

// Float64OrDefault parses str as a float64.
func Float64OrDefault(str string, defaultValue float64) (float64, bool) {
	value, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return defaultValue, false
	}
	return value, true
}

// BoolOrDefault parses str using strconv.ParseBool rules
// ("1", "t", "T", "true", "TRUE", "0", "f", "F", "false", "FALSE").
func BoolOrDefault(str string, defaultValue bool) (bool, bool) {
	value, err := strconv.ParseBool(str)
	if err != nil {
		return defaultValue, false
	}
	return value, true
}
