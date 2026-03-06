package pick

import (
	"testing"
)

// ─── If / IfFunc ────────────────────────────────────────────────────────────

func TestIf_True(t *testing.T) {
	got := If(true, "yes", "no")
	if got != "yes" {
		t.Errorf("If(true) = %q, want %q", got, "yes")
	}
}

func TestIf_False(t *testing.T) {
	got := If(false, "yes", "no")
	if got != "no" {
		t.Errorf("If(false) = %q, want %q", got, "no")
	}
}

func TestIf_IntType(t *testing.T) {
	got := If(1 > 0, 100, 200)
	if got != 100 {
		t.Errorf("If(1>0) = %d, want 100", got)
	}
}

func TestIfFunc_True_OnlyCallsTrueFn(t *testing.T) {
	called := false
	got := IfFunc(true,
		func() string { return "lazy-true" },
		func() string { called = true; return "lazy-false" },
	)
	if got != "lazy-true" {
		t.Errorf("IfFunc(true) = %q, want %q", got, "lazy-true")
	}
	if called {
		t.Error("IfFunc(true) should not call falseFn")
	}
}

func TestIfFunc_False_OnlyCallsFalseFn(t *testing.T) {
	called := false
	got := IfFunc(false,
		func() string { called = true; return "lazy-true" },
		func() string { return "lazy-false" },
	)
	if got != "lazy-false" {
		t.Errorf("IfFunc(false) = %q, want %q", got, "lazy-false")
	}
	if called {
		t.Error("IfFunc(false) should not call trueFn")
	}
}

// ─── OrDefault ──────────────────────────────────────────────────────────────

func TestOrDefault_NonZero(t *testing.T) {
	v, ok := OrDefault("hello", "fallback")
	if v != "hello" || !ok {
		t.Errorf("OrDefault(%q) = (%q, %v), want (%q, true)", "hello", v, ok, "hello")
	}
}

func TestOrDefault_ZeroValue(t *testing.T) {
	v, ok := OrDefault("", "fallback")
	if v != "fallback" || ok {
		t.Errorf("OrDefault(%q) = (%q, %v), want (%q, false)", "", v, ok, "fallback")
	}
}

func TestOrDefault_IntZero(t *testing.T) {
	v, ok := OrDefault(0, 42)
	if v != 42 || ok {
		t.Errorf("OrDefault(0, 42) = (%d, %v), want (42, false)", v, ok)
	}
}

func TestOrDefault_IntNonZero(t *testing.T) {
	v, ok := OrDefault(7, 42)
	if v != 7 || !ok {
		t.Errorf("OrDefault(7, 42) = (%d, %v), want (7, true)", v, ok)
	}
}

// ─── OrDefaultPtr ───────────────────────────────────────────────────────────

func TestOrDefaultPtr_NonNil(t *testing.T) {
	val := 99
	v, ok := OrDefaultPtr(&val, 0)
	if v != 99 || !ok {
		t.Errorf("OrDefaultPtr(&99) = (%d, %v), want (99, true)", v, ok)
	}
}

func TestOrDefaultPtr_Nil(t *testing.T) {
	v, ok := OrDefaultPtr[int](nil, 42)
	if v != 42 || ok {
		t.Errorf("OrDefaultPtr(nil) = (%d, %v), want (42, false)", v, ok)
	}
}

func TestOrDefaultPtr_ZeroPointee(t *testing.T) {
	val := 0
	v, ok := OrDefaultPtr(&val, 99)
	// ptr is non-nil, so we get the pointee (0) with ok=true
	if v != 0 || !ok {
		t.Errorf("OrDefaultPtr(&0) = (%d, %v), want (0, true)", v, ok)
	}
}

// ─── OrDefaultFunc ──────────────────────────────────────────────────────────

func TestOrDefaultFunc_NonZero(t *testing.T) {
	called := false
	v, ok := OrDefaultFunc("present", func() string { called = true; return "default" })
	if v != "present" || !ok || called {
		t.Errorf("OrDefaultFunc(%q) = (%q, %v), called=%v", "present", v, ok, called)
	}
}

func TestOrDefaultFunc_ZeroValue(t *testing.T) {
	v, ok := OrDefaultFunc("", func() string { return "computed" })
	if v != "computed" || ok {
		t.Errorf("OrDefaultFunc(%q) = (%q, %v), want (%q, false)", "", v, ok, "computed")
	}
}

// ─── Coalesce ───────────────────────────────────────────────────────────────

func TestCoalesce_FirstNonZero(t *testing.T) {
	v, ok := Coalesce("", "", "third")
	if v != "third" || !ok {
		t.Errorf("Coalesce = (%q, %v), want (%q, true)", v, ok, "third")
	}
}

func TestCoalesce_AllZero(t *testing.T) {
	v, ok := Coalesce("", "", "")
	if v != "" || ok {
		t.Errorf("Coalesce(all zero) = (%q, %v), want (%q, false)", v, ok, "")
	}
}

func TestCoalesce_Empty(t *testing.T) {
	v, ok := Coalesce[int]()
	if v != 0 || ok {
		t.Errorf("Coalesce() = (%d, %v), want (0, false)", v, ok)
	}
}

func TestCoalesce_FirstArgIsNonZero(t *testing.T) {
	v, ok := Coalesce("first", "second")
	if v != "first" || !ok {
		t.Errorf("Coalesce = (%q, %v), want (%q, true)", v, ok, "first")
	}
}

// ─── MapOrDefault ───────────────────────────────────────────────────────────

func TestMapOrDefault_KeyExists(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	v, ok := MapOrDefault(m, "a", 99)
	if v != 1 || !ok {
		t.Errorf("MapOrDefault(exists) = (%d, %v), want (1, true)", v, ok)
	}
}

func TestMapOrDefault_KeyMissing(t *testing.T) {
	m := map[string]int{"a": 1}
	v, ok := MapOrDefault(m, "z", 99)
	if v != 99 || ok {
		t.Errorf("MapOrDefault(missing) = (%d, %v), want (99, false)", v, ok)
	}
}

func TestMapOrDefault_NilMap(t *testing.T) {
	v, ok := MapOrDefault[string, int](nil, "a", 42)
	if v != 42 || ok {
		t.Errorf("MapOrDefault(nil) = (%d, %v), want (42, false)", v, ok)
	}
}

// ─── SliceFirstOrDefault ────────────────────────────────────────────────────

func TestSliceFirstOrDefault_NonEmpty(t *testing.T) {
	v, ok := SliceFirstOrDefault([]string{"x", "y"}, "default")
	if v != "x" || !ok {
		t.Errorf("SliceFirstOrDefault(nonEmpty) = (%q, %v), want (%q, true)", v, ok, "x")
	}
}

func TestSliceFirstOrDefault_Empty(t *testing.T) {
	v, ok := SliceFirstOrDefault([]string{}, "default")
	if v != "default" || ok {
		t.Errorf("SliceFirstOrDefault(empty) = (%q, %v), want (%q, false)", v, ok, "default")
	}
}

func TestSliceFirstOrDefault_Nil(t *testing.T) {
	v, ok := SliceFirstOrDefault[int](nil, 0)
	if v != 0 || ok {
		t.Errorf("SliceFirstOrDefault(nil) = (%d, %v), want (0, false)", v, ok)
	}
}

// ─── StringOrDefault ────────────────────────────────────────────────────────

func TestStringOrDefault_NonEmpty(t *testing.T) {
	v, ok := StringOrDefault("hello", "def")
	if v != "hello" || !ok {
		t.Errorf("got (%q, %v)", v, ok)
	}
}

func TestStringOrDefault_Empty(t *testing.T) {
	v, ok := StringOrDefault("", "def")
	if v != "def" || ok {
		t.Errorf("got (%q, %v)", v, ok)
	}
}

// ─── IntOrDefault ───────────────────────────────────────────────────────────

func TestIntOrDefault_Valid(t *testing.T) {
	v, ok := IntOrDefault("42", 0)
	if v != 42 || !ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

func TestIntOrDefault_Invalid(t *testing.T) {
	v, ok := IntOrDefault("abc", 99)
	if v != 99 || ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

func TestIntOrDefault_Empty(t *testing.T) {
	v, ok := IntOrDefault("", 7)
	if v != 7 || ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

func TestIntOrDefault_Negative(t *testing.T) {
	v, ok := IntOrDefault("-10", 0)
	if v != -10 || !ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

// ─── Int32OrDefault ─────────────────────────────────────────────────────────

func TestInt32OrDefault_Valid(t *testing.T) {
	v, ok := Int32OrDefault("100", 0)
	if v != 100 || !ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

func TestInt32OrDefault_Overflow(t *testing.T) {
	v, ok := Int32OrDefault("2147483648", 0) // > max int32
	if v != 0 || ok {
		t.Errorf("got (%d, %v), want (0, false)", v, ok)
	}
}

// ─── Int64OrDefault ─────────────────────────────────────────────────────────

func TestInt64OrDefault_Valid(t *testing.T) {
	v, ok := Int64OrDefault("9223372036854775807", 0)
	if v != 9223372036854775807 || !ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

func TestInt64OrDefault_Invalid(t *testing.T) {
	v, ok := Int64OrDefault("not-a-number", -1)
	if v != -1 || ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

// ─── UintOrDefault ──────────────────────────────────────────────────────────

func TestUintOrDefault_Valid(t *testing.T) {
	v, ok := UintOrDefault("100", 0)
	if v != 100 || !ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

func TestUintOrDefault_Negative(t *testing.T) {
	v, ok := UintOrDefault("-1", 0)
	if v != 0 || ok {
		t.Errorf("got (%d, %v), want (0, false)", v, ok)
	}
}

// ─── Uint32OrDefault ────────────────────────────────────────────────────────

func TestUint32OrDefault_Valid(t *testing.T) {
	v, ok := Uint32OrDefault("4294967295", 0) // max uint32
	if v != 4294967295 || !ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

func TestUint32OrDefault_Overflow(t *testing.T) {
	v, ok := Uint32OrDefault("4294967296", 0) // > max uint32
	if v != 0 || ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

// ─── Uint64OrDefault ────────────────────────────────────────────────────────

func TestUint64OrDefault_Valid(t *testing.T) {
	v, ok := Uint64OrDefault("18446744073709551615", 0) // max uint64
	if v != 18446744073709551615 || !ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

func TestUint64OrDefault_Invalid(t *testing.T) {
	v, ok := Uint64OrDefault("xyz", 0)
	if v != 0 || ok {
		t.Errorf("got (%d, %v)", v, ok)
	}
}

// ─── Float32OrDefault ───────────────────────────────────────────────────────

func TestFloat32OrDefault_Valid(t *testing.T) {
	v, ok := Float32OrDefault("3.14", 0)
	if v < 3.13 || v > 3.15 || !ok {
		t.Errorf("got (%f, %v)", v, ok)
	}
}

func TestFloat32OrDefault_Invalid(t *testing.T) {
	v, ok := Float32OrDefault("abc", 1.0)
	if v != 1.0 || ok {
		t.Errorf("got (%f, %v)", v, ok)
	}
}

// ─── Float64OrDefault ───────────────────────────────────────────────────────

func TestFloat64OrDefault_Valid(t *testing.T) {
	v, ok := Float64OrDefault("2.718281828", 0)
	if v < 2.718 || v > 2.719 || !ok {
		t.Errorf("got (%f, %v)", v, ok)
	}
}

func TestFloat64OrDefault_Invalid(t *testing.T) {
	v, ok := Float64OrDefault("nope", 0.0)
	if v != 0.0 || ok {
		t.Errorf("got (%f, %v)", v, ok)
	}
}

// ─── BoolOrDefault ──────────────────────────────────────────────────────────

func TestBoolOrDefault_True(t *testing.T) {
	for _, s := range []string{"1", "t", "T", "true", "TRUE", "True"} {
		v, ok := BoolOrDefault(s, false)
		if v != true || !ok {
			t.Errorf("BoolOrDefault(%q) = (%v, %v), want (true, true)", s, v, ok)
		}
	}
}

func TestBoolOrDefault_False(t *testing.T) {
	for _, s := range []string{"0", "f", "F", "false", "FALSE", "False"} {
		v, ok := BoolOrDefault(s, true)
		if v != false || !ok {
			t.Errorf("BoolOrDefault(%q) = (%v, %v), want (false, true)", s, v, ok)
		}
	}
}

func TestBoolOrDefault_Invalid(t *testing.T) {
	v, ok := BoolOrDefault("maybe", true)
	if v != true || ok {
		t.Errorf("BoolOrDefault(%q) = (%v, %v), want (true, false)", "maybe", v, ok)
	}
}
