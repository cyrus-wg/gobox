package functional

import "iter"

// This file adds Go 1.23+ range-over-function (iter.Seq) helpers alongside the
// slice-based API. Sequences are lazy: no work happens until the returned
// iterator is ranged over or collected, and pipelines fuse without allocating
// intermediate slices.

// Seq converts a slice into an iter.Seq that yields each element in order.
//
//	for v := range functional.Seq([]int{1, 2, 3}) { ... }
func Seq[T any](input []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range input {
			if !yield(v) {
				return
			}
		}
	}
}

// Seq2 converts a slice into an iter.Seq2 that yields (index, value) pairs.
func Seq2[T any](input []T) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, v := range input {
			if !yield(i, v) {
				return
			}
		}
	}
}

// Collect materialises a sequence into a slice. It is a thin, discoverable
// wrapper around slices.Collect.
func Collect[T any](seq iter.Seq[T]) []T {
	var out []T
	for v := range seq {
		out = append(out, v)
	}
	return out
}

// MapSeq lazily applies mapper to each element of seq.
func MapSeq[T, U any](seq iter.Seq[T], mapper func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			if !yield(mapper(v)) {
				return
			}
		}
	}
}

// FilterSeq lazily yields only the elements of seq that satisfy predicate.
func FilterSeq[T any](seq iter.Seq[T], predicate func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if predicate(v) && !yield(v) {
				return
			}
		}
	}
}

// TakeSeq yields at most n elements from seq.
func TakeSeq[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		count := 0
		for v := range seq {
			if !yield(v) {
				return
			}
			count++
			if count >= n {
				return
			}
		}
	}
}
