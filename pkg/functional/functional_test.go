package functional

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Map
// ---------------------------------------------------------------------------

func TestMap(t *testing.T) {
	t.Run("int to string", func(t *testing.T) {
		got := Map([]int{1, 2, 3}, strconv.Itoa)
		want := []string{"1", "2", "3"}
		assertSliceEqual(t, want, got)
	})
	t.Run("empty", func(t *testing.T) {
		got := Map([]int{}, strconv.Itoa)
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %v", got)
		}
	})
	t.Run("double", func(t *testing.T) {
		got := Map([]int{1, 2, 3}, func(n int) int { return n * 2 })
		assertSliceEqual(t, []int{2, 4, 6}, got)
	})
}

// ---------------------------------------------------------------------------
// FlatMap
// ---------------------------------------------------------------------------

func TestFlatMap(t *testing.T) {
	t.Run("split words", func(t *testing.T) {
		got := FlatMap([]string{"hello world", "foo bar"}, func(s string) []string {
			return strings.Split(s, " ")
		})
		assertSliceEqual(t, []string{"hello", "world", "foo", "bar"}, got)
	})
	t.Run("empty", func(t *testing.T) {
		got := FlatMap([]int{}, func(n int) []int { return []int{n, n} })
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
	t.Run("expand", func(t *testing.T) {
		got := FlatMap([]int{1, 2, 3}, func(n int) []int { return []int{n, n * 10} })
		assertSliceEqual(t, []int{1, 10, 2, 20, 3, 30}, got)
	})
}

// ---------------------------------------------------------------------------
// Flatten
// ---------------------------------------------------------------------------

func TestFlatten(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		got := Flatten([][]int{{1, 2}, {3}, {4, 5, 6}})
		assertSliceEqual(t, []int{1, 2, 3, 4, 5, 6}, got)
	})
	t.Run("empty inner", func(t *testing.T) {
		got := Flatten([][]int{{}, {1}, {}})
		assertSliceEqual(t, []int{1}, got)
	})
	t.Run("empty outer", func(t *testing.T) {
		got := Flatten([][]int{})
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// Filter
// ---------------------------------------------------------------------------

func TestFilter(t *testing.T) {
	t.Run("even", func(t *testing.T) {
		got := Filter([]int{1, 2, 3, 4, 5, 6}, func(n int) bool { return n%2 == 0 })
		assertSliceEqual(t, []int{2, 4, 6}, got)
	})
	t.Run("none match", func(t *testing.T) {
		got := Filter([]int{1, 3, 5}, func(n int) bool { return n%2 == 0 })
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
	t.Run("all match", func(t *testing.T) {
		got := Filter([]int{2, 4}, func(n int) bool { return n%2 == 0 })
		assertSliceEqual(t, []int{2, 4}, got)
	})
}

// ---------------------------------------------------------------------------
// Find
// ---------------------------------------------------------------------------

func TestFind(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		val, ok := Find([]int{1, 2, 3, 4}, func(n int) bool { return n > 2 })
		if !ok || val != 3 {
			t.Errorf("expected (3, true), got (%d, %v)", val, ok)
		}
	})
	t.Run("not found", func(t *testing.T) {
		_, ok := Find([]int{1, 2}, func(n int) bool { return n > 5 })
		if ok {
			t.Error("expected not found")
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, ok := Find([]int{}, func(n int) bool { return true })
		if ok {
			t.Error("expected not found for empty slice")
		}
	})
}

// ---------------------------------------------------------------------------
// FindIndex
// ---------------------------------------------------------------------------

func TestFindIndex(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		idx := FindIndex([]string{"a", "b", "c"}, func(s string) bool { return s == "b" })
		if idx != 1 {
			t.Errorf("expected 1, got %d", idx)
		}
	})
	t.Run("not found", func(t *testing.T) {
		idx := FindIndex([]string{"a"}, func(s string) bool { return s == "z" })
		if idx != -1 {
			t.Errorf("expected -1, got %d", idx)
		}
	})
}

// ---------------------------------------------------------------------------
// Contains
// ---------------------------------------------------------------------------

func TestContains(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		if !Contains([]int{1, 2, 3}, 2) {
			t.Error("expected true")
		}
	})
	t.Run("absent", func(t *testing.T) {
		if Contains([]int{1, 2, 3}, 4) {
			t.Error("expected false")
		}
	})
	t.Run("empty", func(t *testing.T) {
		if Contains([]int{}, 1) {
			t.Error("expected false for empty slice")
		}
	})
}

// ---------------------------------------------------------------------------
// IndexOf
// ---------------------------------------------------------------------------

func TestIndexOf(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		if idx := IndexOf([]string{"a", "b", "c"}, "b"); idx != 1 {
			t.Errorf("expected 1, got %d", idx)
		}
	})
	t.Run("absent", func(t *testing.T) {
		if idx := IndexOf([]string{"a"}, "z"); idx != -1 {
			t.Errorf("expected -1, got %d", idx)
		}
	})
}

// ---------------------------------------------------------------------------
// Reduce
// ---------------------------------------------------------------------------

func TestReduce(t *testing.T) {
	t.Run("sum", func(t *testing.T) {
		got := Reduce([]int{1, 2, 3, 4}, func(acc, n int) int { return acc + n }, 0)
		if got != 10 {
			t.Errorf("expected 10, got %d", got)
		}
	})
	t.Run("concat", func(t *testing.T) {
		got := Reduce([]string{"a", "b", "c"}, func(acc, s string) string { return acc + s }, "")
		if got != "abc" {
			t.Errorf("expected abc, got %s", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		got := Reduce([]int{}, func(acc, n int) int { return acc + n }, 42)
		if got != 42 {
			t.Errorf("expected initial value 42, got %d", got)
		}
	})
}

// ---------------------------------------------------------------------------
// Sum / SumBy
// ---------------------------------------------------------------------------

func TestSum(t *testing.T) {
	t.Run("ints", func(t *testing.T) {
		if got := Sum([]int{1, 2, 3}); got != 6 {
			t.Errorf("expected 6, got %d", got)
		}
	})
	t.Run("floats", func(t *testing.T) {
		if got := Sum([]float64{1.5, 2.5}); got != 4.0 {
			t.Errorf("expected 4.0, got %f", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if got := Sum([]int{}); got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})
}

func TestSumBy(t *testing.T) {
	type item struct {
		name  string
		price int
	}
	items := []item{{"a", 10}, {"b", 20}, {"c", 30}}
	got := SumBy(items, func(i item) int { return i.price })
	if got != 60 {
		t.Errorf("expected 60, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Count
// ---------------------------------------------------------------------------

func TestCount(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		got := Count([]int{1, 2, 3, 4, 5}, func(n int) bool { return n > 3 })
		if got != 2 {
			t.Errorf("expected 2, got %d", got)
		}
	})
	t.Run("none", func(t *testing.T) {
		got := Count([]int{1, 2}, func(n int) bool { return n > 10 })
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})
}

// ---------------------------------------------------------------------------
// MinBy / MaxBy / Min / Max
// ---------------------------------------------------------------------------

func TestMinBy(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		val, ok := MinBy([]int{3, 1, 4, 1, 5}, cmp.Compare[int])
		if !ok || val != 1 {
			t.Errorf("expected (1, true), got (%d, %v)", val, ok)
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, ok := MinBy([]int{}, cmp.Compare[int])
		if ok {
			t.Error("expected false for empty slice")
		}
	})
}

func TestMaxBy(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		val, ok := MaxBy([]int{3, 1, 4, 1, 5}, cmp.Compare[int])
		if !ok || val != 5 {
			t.Errorf("expected (5, true), got (%d, %v)", val, ok)
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, ok := MaxBy([]int{}, cmp.Compare[int])
		if ok {
			t.Error("expected false for empty slice")
		}
	})
}

func TestMin(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		val, ok := Min([]int{5, 2, 8, 1})
		if !ok || val != 1 {
			t.Errorf("expected (1, true), got (%d, %v)", val, ok)
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, ok := Min([]int{})
		if ok {
			t.Error("expected false for empty slice")
		}
	})
}

func TestMax(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		val, ok := Max([]int{5, 2, 8, 1})
		if !ok || val != 8 {
			t.Errorf("expected (8, true), got (%d, %v)", val, ok)
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, ok := Max([]int{})
		if ok {
			t.Error("expected false for empty slice")
		}
	})
}

// ---------------------------------------------------------------------------
// Any / All / None
// ---------------------------------------------------------------------------

func TestAny(t *testing.T) {
	if !Any([]int{1, 2, 3}, func(n int) bool { return n == 2 }) {
		t.Error("expected true")
	}
	if Any([]int{1, 2, 3}, func(n int) bool { return n == 5 }) {
		t.Error("expected false")
	}
	if Any([]int{}, func(n int) bool { return true }) {
		t.Error("expected false for empty")
	}
}

func TestAll(t *testing.T) {
	if !All([]int{2, 4, 6}, func(n int) bool { return n%2 == 0 }) {
		t.Error("expected true")
	}
	if All([]int{2, 3, 6}, func(n int) bool { return n%2 == 0 }) {
		t.Error("expected false")
	}
	if !All([]int{}, func(n int) bool { return false }) {
		t.Error("All on empty should be true")
	}
}

func TestNone(t *testing.T) {
	if !None([]int{1, 3, 5}, func(n int) bool { return n%2 == 0 }) {
		t.Error("expected true")
	}
	if None([]int{1, 2, 3}, func(n int) bool { return n%2 == 0 }) {
		t.Error("expected false")
	}
	if !None([]int{}, func(n int) bool { return true }) {
		t.Error("None on empty should be true")
	}
}

// ---------------------------------------------------------------------------
// ForEach / ForEachIndexed
// ---------------------------------------------------------------------------

func TestForEach(t *testing.T) {
	sum := 0
	ForEach([]int{1, 2, 3}, func(n int) { sum += n })
	if sum != 6 {
		t.Errorf("expected 6, got %d", sum)
	}
}

func TestForEachIndexed(t *testing.T) {
	indices := make([]int, 0)
	values := make([]string, 0)
	ForEachIndexed([]string{"a", "b"}, func(i int, s string) {
		indices = append(indices, i)
		values = append(values, s)
	})
	assertSliceEqual(t, []int{0, 1}, indices)
	assertSliceEqual(t, []string{"a", "b"}, values)
}

// ---------------------------------------------------------------------------
// Distinct / DistinctBy
// ---------------------------------------------------------------------------

func TestDistinct(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		got := Distinct([]int{1, 2, 2, 3, 1, 3})
		assertSliceEqual(t, []int{1, 2, 3}, got)
	})
	t.Run("empty", func(t *testing.T) {
		got := Distinct([]int{})
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
	t.Run("no duplicates", func(t *testing.T) {
		got := Distinct([]int{1, 2, 3})
		assertSliceEqual(t, []int{1, 2, 3}, got)
	})
}

func TestDistinctBy(t *testing.T) {
	type person struct {
		name string
		age  int
	}
	people := []person{{"Alice", 30}, {"Bob", 25}, {"Carol", 30}}
	got := DistinctBy(people, func(p person) int { return p.age })
	if len(got) != 2 {
		t.Errorf("expected 2 distinct ages, got %d", len(got))
	}
	if got[0].name != "Alice" || got[1].name != "Bob" {
		t.Errorf("unexpected result: %v", got)
	}
}

// ---------------------------------------------------------------------------
// GroupBy
// ---------------------------------------------------------------------------

func TestGroupBy(t *testing.T) {
	got := GroupBy([]int{1, 2, 3, 4, 5, 6}, func(n int) string {
		if n%2 == 0 {
			return "even"
		}
		return "odd"
	})
	assertSliceEqual(t, []int{1, 3, 5}, got["odd"])
	assertSliceEqual(t, []int{2, 4, 6}, got["even"])
}

// ---------------------------------------------------------------------------
// Associate / ToMap
// ---------------------------------------------------------------------------

func TestAssociate(t *testing.T) {
	got := Associate([]string{"hello", "world"}, func(s string) (int, string) {
		return len(s), s
	})
	if got[5] != "world" { // both len 5, last wins
		t.Errorf("unexpected: %v", got)
	}
}

func TestToMap(t *testing.T) {
	type user struct {
		id   int
		name string
	}
	users := []user{{1, "Alice"}, {2, "Bob"}}
	got := ToMap(users, func(u user) int { return u.id })
	if got[1].name != "Alice" || got[2].name != "Bob" {
		t.Errorf("unexpected: %v", got)
	}
}

// ---------------------------------------------------------------------------
// Keys / Values
// ---------------------------------------------------------------------------

func TestKeys(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	got := Keys(m)
	slices.Sort(got)
	assertSliceEqual(t, []string{"a", "b", "c"}, got)
}

func TestValues(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	got := Values(m)
	slices.Sort(got)
	assertSliceEqual(t, []int{1, 2, 3}, got)
}

// ---------------------------------------------------------------------------
// Partition
// ---------------------------------------------------------------------------

func TestPartition(t *testing.T) {
	truePart, falsePart := Partition([]int{1, 2, 3, 4, 5}, func(n int) bool { return n%2 == 0 })
	assertSliceEqual(t, []int{2, 4}, truePart)
	assertSliceEqual(t, []int{1, 3, 5}, falsePart)
}

// ---------------------------------------------------------------------------
// Chunk
// ---------------------------------------------------------------------------

func TestChunk(t *testing.T) {
	t.Run("even split", func(t *testing.T) {
		got := Chunk([]int{1, 2, 3, 4}, 2)
		if len(got) != 2 {
			t.Fatalf("expected 2 chunks, got %d", len(got))
		}
		assertSliceEqual(t, []int{1, 2}, got[0])
		assertSliceEqual(t, []int{3, 4}, got[1])
	})
	t.Run("remainder", func(t *testing.T) {
		got := Chunk([]int{1, 2, 3, 4, 5}, 2)
		if len(got) != 3 {
			t.Fatalf("expected 3 chunks, got %d", len(got))
		}
		assertSliceEqual(t, []int{5}, got[2])
	})
	t.Run("size zero", func(t *testing.T) {
		got := Chunk([]int{1, 2}, 0)
		if got != nil {
			t.Errorf("expected nil for size 0, got %v", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		got := Chunk([]int{}, 3)
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// Take / TakeWhile
// ---------------------------------------------------------------------------

func TestTake(t *testing.T) {
	t.Run("less than len", func(t *testing.T) {
		assertSliceEqual(t, []int{1, 2}, Take([]int{1, 2, 3, 4}, 2))
	})
	t.Run("more than len", func(t *testing.T) {
		assertSliceEqual(t, []int{1, 2}, Take([]int{1, 2}, 5))
	})
	t.Run("zero", func(t *testing.T) {
		got := Take([]int{1, 2, 3}, 0)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("negative", func(t *testing.T) {
		got := Take([]int{1, 2, 3}, -1)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

func TestTakeWhile(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		got := TakeWhile([]int{1, 2, 3, 4, 1}, func(n int) bool { return n < 3 })
		assertSliceEqual(t, []int{1, 2}, got)
	})
	t.Run("all match", func(t *testing.T) {
		got := TakeWhile([]int{1, 2}, func(n int) bool { return n < 10 })
		assertSliceEqual(t, []int{1, 2}, got)
	})
	t.Run("none match", func(t *testing.T) {
		got := TakeWhile([]int{5, 6}, func(n int) bool { return n < 3 })
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// Drop / DropWhile
// ---------------------------------------------------------------------------

func TestDrop(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		assertSliceEqual(t, []int{3, 4}, Drop([]int{1, 2, 3, 4}, 2))
	})
	t.Run("more than len", func(t *testing.T) {
		got := Drop([]int{1, 2}, 5)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("zero", func(t *testing.T) {
		assertSliceEqual(t, []int{1, 2}, Drop([]int{1, 2}, 0))
	})
}

func TestDropWhile(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		got := DropWhile([]int{1, 2, 3, 4}, func(n int) bool { return n < 3 })
		assertSliceEqual(t, []int{3, 4}, got)
	})
	t.Run("all match", func(t *testing.T) {
		got := DropWhile([]int{1, 2}, func(n int) bool { return n < 10 })
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("none match", func(t *testing.T) {
		got := DropWhile([]int{5, 6}, func(n int) bool { return n < 3 })
		assertSliceEqual(t, []int{5, 6}, got)
	})
}

// ---------------------------------------------------------------------------
// Reverse
// ---------------------------------------------------------------------------

func TestReverse(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		assertSliceEqual(t, []int{3, 2, 1}, Reverse([]int{1, 2, 3}))
	})
	t.Run("does not mutate original", func(t *testing.T) {
		orig := []int{1, 2, 3}
		_ = Reverse(orig)
		assertSliceEqual(t, []int{1, 2, 3}, orig)
	})
	t.Run("empty", func(t *testing.T) {
		got := Reverse([]int{})
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// SortBy
// ---------------------------------------------------------------------------

func TestSortBy(t *testing.T) {
	t.Run("ascending", func(t *testing.T) {
		got := SortBy([]int{3, 1, 2}, cmp.Compare[int])
		assertSliceEqual(t, []int{1, 2, 3}, got)
	})
	t.Run("descending", func(t *testing.T) {
		got := SortBy([]int{1, 3, 2}, func(a, b int) int { return cmp.Compare(b, a) })
		assertSliceEqual(t, []int{3, 2, 1}, got)
	})
	t.Run("does not mutate original", func(t *testing.T) {
		orig := []int{3, 1, 2}
		_ = SortBy(orig, cmp.Compare[int])
		assertSliceEqual(t, []int{3, 1, 2}, orig)
	})
}

// ---------------------------------------------------------------------------
// Zip
// ---------------------------------------------------------------------------

func TestZip(t *testing.T) {
	t.Run("equal length", func(t *testing.T) {
		got := Zip([]int{1, 2}, []string{"a", "b"})
		if len(got) != 2 {
			t.Fatalf("expected 2 pairs, got %d", len(got))
		}
		if got[0] != [2]any{1, "a"} || got[1] != [2]any{2, "b"} {
			t.Errorf("unexpected: %v", got)
		}
	})
	t.Run("unequal length", func(t *testing.T) {
		got := Zip([]int{1, 2, 3}, []string{"a"})
		if len(got) != 1 {
			t.Fatalf("expected 1 pair, got %d", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// ZipWith
// ---------------------------------------------------------------------------

func TestZipWith(t *testing.T) {
	got := ZipWith([]int{1, 2, 3}, []int{10, 20, 30}, func(a, b int) int { return a + b })
	assertSliceEqual(t, []int{11, 22, 33}, got)
}

// ---------------------------------------------------------------------------
// UnzipPairs
// ---------------------------------------------------------------------------

func TestUnzip(t *testing.T) {
	pairs := []Pair[string, int]{
		{"a", 1},
		{"b", 2},
		{"c", 3},
	}
	keys, vals := Unzip(pairs)
	assertSliceEqual(t, []string{"a", "b", "c"}, keys)
	assertSliceEqual(t, []int{1, 2, 3}, vals)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertSliceEqual[T comparable](t *testing.T, want, got []T) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("length mismatch: want %d, got %d\nwant: %v\ngot:  %v", len(want), len(got), want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("index %d: want %v, got %v", i, want[i], got[i])
		}
	}
}
