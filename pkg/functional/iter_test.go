package functional

import (
	"slices"
	"testing"
)

func TestSeq_And_Collect(t *testing.T) {
	got := Collect(Seq([]int{1, 2, 3}))
	assertSliceEqual(t, []int{1, 2, 3}, got)
}

func TestSeq2_IndexValue(t *testing.T) {
	var indices []int
	var values []string
	for i, v := range Seq2([]string{"a", "b"}) {
		indices = append(indices, i)
		values = append(values, v)
	}
	assertSliceEqual(t, []int{0, 1}, indices)
	assertSliceEqual(t, []string{"a", "b"}, values)
}

func TestMapSeq(t *testing.T) {
	got := Collect(MapSeq(Seq([]int{1, 2, 3}), func(n int) int { return n * 10 }))
	assertSliceEqual(t, []int{10, 20, 30}, got)
}

func TestFilterSeq(t *testing.T) {
	got := Collect(FilterSeq(Seq([]int{1, 2, 3, 4}), func(n int) bool { return n%2 == 0 }))
	assertSliceEqual(t, []int{2, 4}, got)
}

func TestTakeSeq(t *testing.T) {
	got := Collect(TakeSeq(Seq([]int{1, 2, 3, 4, 5}), 3))
	assertSliceEqual(t, []int{1, 2, 3}, got)

	if n := len(Collect(TakeSeq(Seq([]int{1, 2}), 0))); n != 0 {
		t.Fatalf("expected empty for n=0, got %d", n)
	}
}

func TestSeqPipeline_Fused(t *testing.T) {
	// A fused pipeline: map -> filter -> take, collected at the end.
	seq := TakeSeq(
		FilterSeq(
			MapSeq(Seq([]int{1, 2, 3, 4, 5, 6}), func(n int) int { return n * n }),
			func(n int) bool { return n%2 == 0 },
		),
		2,
	)
	got := Collect(seq)
	assertSliceEqual(t, []int{4, 16}, got)
}

func TestFilterSeq_EarlyStop(t *testing.T) {
	// Ranging with an early break must stop the producer.
	seen := 0
	for v := range FilterSeq(Seq([]int{1, 2, 3, 4, 5}), func(int) bool { return true }) {
		seen++
		if v == 2 {
			break
		}
	}
	if seen != 2 {
		t.Fatalf("expected to stop after 2 elements, saw %d", seen)
	}
}

func TestSeq_StdlibInterop(t *testing.T) {
	// Our Seq should interoperate with the standard slices.Collect.
	got := slices.Collect(Seq([]int{9, 8, 7}))
	assertSliceEqual(t, []int{9, 8, 7}, got)
}
