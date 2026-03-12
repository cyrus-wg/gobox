package functional

import (
	"cmp"
	"slices"
)

// Numeric is a constraint for types that support arithmetic operations.
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// ---------------------------------------------------------------------------
// Transformations
// ---------------------------------------------------------------------------

// Map applies a mapper function to each element of the input slice,
// returning a new slice of the mapped results.
func Map[T any, U any](input []T, mapper func(T) U) []U {
	result := make([]U, len(input))
	for i, v := range input {
		result[i] = mapper(v)
	}
	return result
}

// FlatMap applies a mapper that returns a slice for each element,
// then flattens the results into a single slice.
func FlatMap[T any, U any](input []T, mapper func(T) []U) []U {
	result := make([]U, 0)
	for _, v := range input {
		result = append(result, mapper(v)...)
	}
	return result
}

// Flatten concatenates a slice of slices into a single flat slice.
func Flatten[T any](input [][]T) []T {
	result := make([]T, 0)
	for _, inner := range input {
		result = append(result, inner...)
	}
	return result
}

// ---------------------------------------------------------------------------
// Filtering / Selection
// ---------------------------------------------------------------------------

// Filter returns a new slice containing only elements that satisfy the predicate.
func Filter[T any](input []T, predicate func(T) bool) []T {
	result := make([]T, 0)
	for _, v := range input {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// Find returns the first element satisfying the predicate and true,
// or the zero value and false if none is found.
func Find[T any](input []T, predicate func(T) bool) (T, bool) {
	for _, v := range input {
		if predicate(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// FindIndex returns the index of the first element satisfying the predicate,
// or -1 if none is found.
func FindIndex[T any](input []T, predicate func(T) bool) int {
	return slices.IndexFunc(input, predicate)
}

// Contains reports whether the slice contains the given value.
func Contains[T comparable](input []T, value T) bool {
	return slices.Contains(input, value)
}

// IndexOf returns the index of the first occurrence of value, or -1 if not found.
func IndexOf[T comparable](input []T, value T) int {
	return slices.Index(input, value)
}

// ---------------------------------------------------------------------------
// Aggregation / Reduction
// ---------------------------------------------------------------------------

// Reduce folds the input slice into a single value using the reducer function.
func Reduce[T any, U any](input []T, reducer func(U, T) U, initial U) U {
	result := initial
	for _, v := range input {
		result = reducer(result, v)
	}
	return result
}

// Sum returns the sum of all elements in a numeric slice.
func Sum[T Numeric](input []T) T {
	var total T
	for _, v := range input {
		total += v
	}
	return total
}

// SumBy returns the sum of the values extracted by the selector function.
func SumBy[T any, N Numeric](input []T, selector func(T) N) N {
	var total N
	for _, v := range input {
		total += selector(v)
	}
	return total
}

// Count returns the number of elements satisfying the predicate.
func Count[T any](input []T, predicate func(T) bool) int {
	count := 0
	for _, v := range input {
		if predicate(v) {
			count++
		}
	}
	return count
}

// MinBy returns the minimum element according to the comparison function.
// Returns the zero value and false if the slice is empty.
func MinBy[T any](input []T, less func(a, b T) int) (T, bool) {
	if len(input) == 0 {
		var zero T
		return zero, false
	}
	m := input[0]
	for _, v := range input[1:] {
		if less(v, m) < 0 {
			m = v
		}
	}
	return m, true
}

// MaxBy returns the maximum element according to the comparison function.
// Returns the zero value and false if the slice is empty.
func MaxBy[T any](input []T, less func(a, b T) int) (T, bool) {
	if len(input) == 0 {
		var zero T
		return zero, false
	}
	m := input[0]
	for _, v := range input[1:] {
		if less(v, m) > 0 {
			m = v
		}
	}
	return m, true
}

// Min returns the minimum element of an ordered slice.
// Returns the zero value and false if the slice is empty.
func Min[T cmp.Ordered](input []T) (T, bool) {
	if len(input) == 0 {
		var zero T
		return zero, false
	}
	return slices.Min(input), true
}

// Max returns the maximum element of an ordered slice.
// Returns the zero value and false if the slice is empty.
func Max[T cmp.Ordered](input []T) (T, bool) {
	if len(input) == 0 {
		var zero T
		return zero, false
	}
	return slices.Max(input), true
}

// ---------------------------------------------------------------------------
// Predicates
// ---------------------------------------------------------------------------

// Any reports whether at least one element satisfies the predicate.
func Any[T any](input []T, predicate func(T) bool) bool {
	return slices.ContainsFunc(input, predicate)
}

// All reports whether every element satisfies the predicate.
// Returns true for an empty slice.
func All[T any](input []T, predicate func(T) bool) bool {
	return slices.IndexFunc(input, func(v T) bool { return !predicate(v) }) == -1
}

// None reports whether no elements satisfy the predicate.
// Returns true for an empty slice.
func None[T any](input []T, predicate func(T) bool) bool {
	return !slices.ContainsFunc(input, predicate)
}

// ---------------------------------------------------------------------------
// Side Effects
// ---------------------------------------------------------------------------

// ForEach invokes the action function for each element.
func ForEach[T any](input []T, action func(T)) {
	for _, v := range input {
		action(v)
	}
}

// ForEachIndexed invokes the action function for each element with its index.
func ForEachIndexed[T any](input []T, action func(int, T)) {
	for i, v := range input {
		action(i, v)
	}
}

// ---------------------------------------------------------------------------
// Deduplication
// ---------------------------------------------------------------------------

// Distinct returns a new slice with duplicate elements removed.
// Preserves the order of first occurrence.
func Distinct[T comparable](input []T) []T {
	seen := make(map[T]struct{})
	result := make([]T, 0)
	for _, v := range input {
		if _, exists := seen[v]; !exists {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// DistinctBy returns a new slice with duplicates removed based on a key selector.
// Preserves the order of first occurrence.
func DistinctBy[T any, K comparable](input []T, keySelector func(T) K) []T {
	seen := make(map[K]struct{})
	result := make([]T, 0)
	for _, v := range input {
		key := keySelector(v)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Grouping / Mapping
// ---------------------------------------------------------------------------

// GroupBy groups elements by a key, returning a map of key → slice of elements.
func GroupBy[T any, K comparable](input []T, keySelector func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, v := range input {
		key := keySelector(v)
		result[key] = append(result[key], v)
	}
	return result
}

// Associate builds a map from the input slice using a function that returns
// a key-value pair for each element.
func Associate[T any, K comparable, V any](input []T, transform func(T) (K, V)) map[K]V {
	result := make(map[K]V, len(input))
	for _, v := range input {
		k, val := transform(v)
		result[k] = val
	}
	return result
}

// ToMap builds a map by extracting keys from each element; the values are the elements themselves.
func ToMap[T any, K comparable](input []T, keySelector func(T) K) map[K]T {
	result := make(map[K]T, len(input))
	for _, v := range input {
		result[keySelector(v)] = v
	}
	return result
}

// Keys returns all keys of a map as a slice (order is not guaranteed).
func Keys[K comparable, V any](m map[K]V) []K {
	result := make([]K, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

// Values returns all values of a map as a slice (order is not guaranteed).
func Values[K comparable, V any](m map[K]V) []V {
	result := make([]V, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

// ---------------------------------------------------------------------------
// Partitioning / Chunking
// ---------------------------------------------------------------------------

// Partition splits the input into two slices: elements that satisfy the
// predicate and elements that do not.
func Partition[T any](input []T, predicate func(T) bool) (truePart []T, falsePart []T) {
	for _, v := range input {
		if predicate(v) {
			truePart = append(truePart, v)
		} else {
			falsePart = append(falsePart, v)
		}
	}
	return
}

// Chunk splits the input slice into sub-slices of the given size.
// The last chunk may have fewer elements.
func Chunk[T any](input []T, size int) [][]T {
	if size <= 0 {
		return nil
	}
	result := make([][]T, 0, (len(input)+size-1)/size)
	for i := 0; i < len(input); i += size {
		end := min(i+size, len(input))
		result = append(result, input[i:end])
	}
	return result
}

// ---------------------------------------------------------------------------
// Slicing
// ---------------------------------------------------------------------------

// Take returns the first n elements (or all if n > len).
func Take[T any](input []T, n int) []T {
	if n <= 0 {
		return nil
	}
	if n >= len(input) {
		return slices.Clone(input)
	}
	return slices.Clone(input[:n])
}

// TakeWhile returns the longest prefix of elements satisfying the predicate.
func TakeWhile[T any](input []T, predicate func(T) bool) []T {
	for i, v := range input {
		if !predicate(v) {
			return slices.Clone(input[:i])
		}
	}
	return slices.Clone(input)
}

// Drop returns the input slice with the first n elements removed.
func Drop[T any](input []T, n int) []T {
	if n <= 0 {
		return slices.Clone(input)
	}
	if n >= len(input) {
		return nil
	}
	return slices.Clone(input[n:])
}

// DropWhile skips elements while the predicate holds, then returns the rest.
func DropWhile[T any](input []T, predicate func(T) bool) []T {
	for i, v := range input {
		if !predicate(v) {
			return slices.Clone(input[i:])
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Ordering
// ---------------------------------------------------------------------------

// Reverse returns a new slice with elements in reverse order.
func Reverse[T any](input []T) []T {
	result := slices.Clone(input)
	slices.Reverse(result)
	return result
}

// SortBy returns a new sorted slice using the provided comparison function.
// The cmp function should return a negative number when a < b,
// zero when a == b, and a positive number when a > b.
func SortBy[T any](input []T, cmpFn func(a, b T) int) []T {
	result := slices.Clone(input)
	slices.SortFunc(result, cmpFn)
	return result
}

// ---------------------------------------------------------------------------
// Zipping
// ---------------------------------------------------------------------------

// Zip pairs elements from two slices, truncated to the shorter length.
func Zip[T any, U any](a []T, b []U) [][2]any {
	length := min(len(a), len(b))
	result := make([][2]any, length)
	for i := range length {
		result[i] = [2]any{a[i], b[i]}
	}
	return result
}

// ZipWith combines elements from two slices using a combiner function,
// truncated to the shorter length. This is the type-safe alternative to Zip.
func ZipWith[T any, U any, R any](a []T, b []U, combiner func(T, U) R) []R {
	length := min(len(a), len(b))
	result := make([]R, length)
	for i := range length {
		result[i] = combiner(a[i], b[i])
	}
	return result
}

// Pair is a generic pair type.
type Pair[T any, U any] struct {
	First  T
	Second U
}

// Unzip splits a slice of Pair into two separate slices.
func Unzip[T any, U any](input []Pair[T, U]) ([]T, []U) {
	ts := make([]T, len(input))
	us := make([]U, len(input))
	for i, pair := range input {
		ts[i] = pair.First
		us[i] = pair.Second
	}
	return ts, us
}
