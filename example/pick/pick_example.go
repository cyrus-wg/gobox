package main

import (
	"fmt"

	"github.com/cyrus-wg/gobox/pkg/pick"
)

func main() {
	fmt.Println("=== pick package examples ===")
	fmt.Println()

	// ---------------------------------------------------------------
	// 1. If / IfFunc — generic ternary operators
	// ---------------------------------------------------------------
	fmt.Println("// 1. If / IfFunc — ternary helpers")

	label := pick.If(true, "yes", "no")
	fmt.Println("pick.If(true, \"yes\", \"no\")  →", label) // yes

	label = pick.If(false, "yes", "no")
	fmt.Println("pick.If(false, \"yes\", \"no\") →", label) // no

	// IfFunc: lazy evaluation — only the chosen branch's function is called.
	calls := 0
	trueFn := func() string { calls++; return "expensive-true" }
	falseFn := func() string { calls++; return "expensive-false" }

	result := pick.IfFunc(true, trueFn, falseFn)
	fmt.Printf("pick.IfFunc(true, ...)  → %q  (calls=%d)\n", result, calls) // 1

	result = pick.IfFunc(false, trueFn, falseFn)
	fmt.Printf("pick.IfFunc(false, ...) → %q (calls=%d)\n", result, calls) // 2
	fmt.Println()

	// ---------------------------------------------------------------
	// 2. OrDefault — return value if non-zero, else fallback
	// ---------------------------------------------------------------
	fmt.Println("// 2. OrDefault")

	val, ok := pick.OrDefault("", "fallback")
	fmt.Printf("pick.OrDefault(\"\", \"fallback\")      → %q, ok=%v\n", val, ok)

	val, ok = pick.OrDefault("hello", "fallback")
	fmt.Printf("pick.OrDefault(\"hello\", \"fallback\") → %q, ok=%v\n", val, ok)
	fmt.Println()

	// ---------------------------------------------------------------
	// 3. OrDefaultPtr — dereference pointer or use fallback
	// ---------------------------------------------------------------
	fmt.Println("// 3. OrDefaultPtr")

	timeout := 60
	tv, tok := pick.OrDefaultPtr(&timeout, 30)
	fmt.Printf("pick.OrDefaultPtr(&60, 30)  → %d, ok=%v\n", tv, tok)

	tv, tok = pick.OrDefaultPtr[int](nil, 30)
	fmt.Printf("pick.OrDefaultPtr(nil, 30)  → %d, ok=%v\n", tv, tok)
	fmt.Println()

	// ---------------------------------------------------------------
	// 4. OrDefaultFunc — lazy default
	// ---------------------------------------------------------------
	fmt.Println("// 4. OrDefaultFunc")

	expensiveCalls := 0
	expensiveDefault := func() string { expensiveCalls++; return "computed-default" }

	v, ok2 := pick.OrDefaultFunc("", expensiveDefault)
	fmt.Printf("pick.OrDefaultFunc(\"\", fn)  → %q, ok=%v (fn called: %d)\n", v, ok2, expensiveCalls)

	v, ok2 = pick.OrDefaultFunc("present", expensiveDefault)
	fmt.Printf("pick.OrDefaultFunc(\"present\", fn) → %q, ok=%v (fn called: %d)\n", v, ok2, expensiveCalls)
	fmt.Println()

	// ---------------------------------------------------------------
	// 5. Coalesce — first non-zero value
	// ---------------------------------------------------------------
	fmt.Println("// 5. Coalesce")

	cv, cok := pick.Coalesce("", "", "third")
	fmt.Printf("pick.Coalesce(\"\", \"\", \"third\") → %q, ok=%v\n", cv, cok)

	cv, cok = pick.Coalesce("first", "second", "third")
	fmt.Printf("pick.Coalesce(\"first\", \"second\", \"third\") → %q, ok=%v\n", cv, cok)

	cv, cok = pick.Coalesce("", "", "")
	fmt.Printf("pick.Coalesce(\"\", \"\", \"\") → %q, ok=%v\n", cv, cok)
	fmt.Println()

	// ---------------------------------------------------------------
	// 6. MapOrDefault — safe map lookup
	// ---------------------------------------------------------------
	fmt.Println("// 6. MapOrDefault")

	roles := map[string]string{"alice": "admin", "bob": "viewer"}
	mv, mok := pick.MapOrDefault(roles, "alice", "guest")
	fmt.Printf("pick.MapOrDefault(roles, \"alice\", \"guest\") → %q, ok=%v\n", mv, mok)

	mv, mok = pick.MapOrDefault(roles, "unknown", "guest")
	fmt.Printf("pick.MapOrDefault(roles, \"unknown\", \"guest\") → %q, ok=%v\n", mv, mok)
	fmt.Println()

	// ---------------------------------------------------------------
	// 7. SliceFirstOrDefault — first element or fallback
	// ---------------------------------------------------------------
	fmt.Println("// 7. SliceFirstOrDefault")

	addrs := []string{"10.0.0.1", "10.0.0.2"}
	sv, sok := pick.SliceFirstOrDefault(addrs, "127.0.0.1")
	fmt.Printf("pick.SliceFirstOrDefault([\"10.0.0.1\", ...], \"127.0.0.1\") → %q, ok=%v\n", sv, sok)

	sv, sok = pick.SliceFirstOrDefault([]string{}, "127.0.0.1")
	fmt.Printf("pick.SliceFirstOrDefault([], \"127.0.0.1\") → %q, ok=%v\n", sv, sok)
	fmt.Println()

	// ---------------------------------------------------------------
	// 8. String/number parsers
	// ---------------------------------------------------------------
	fmt.Println("// 8. String-to-type parsers")

	si, siOk := pick.StringOrDefault("hello", "default")
	fmt.Printf("pick.StringOrDefault(\"hello\", \"default\") → %q, ok=%v\n", si, siOk)

	si, siOk = pick.StringOrDefault("", "default")
	fmt.Printf("pick.StringOrDefault(\"\", \"default\")     → %q, ok=%v\n", si, siOk)

	iv, iok := pick.IntOrDefault("42", 0)
	fmt.Printf("pick.IntOrDefault(\"42\", 0)  → %d, ok=%v\n", iv, iok)

	iv, iok = pick.IntOrDefault("abc", 0)
	fmt.Printf("pick.IntOrDefault(\"abc\", 0) → %d, ok=%v\n", iv, iok)

	i32, i32ok := pick.Int32OrDefault("100", 0)
	fmt.Printf("pick.Int32OrDefault(\"100\", 0)  → %d, ok=%v\n", i32, i32ok)

	i64, i64ok := pick.Int64OrDefault("9999999999", 0)
	fmt.Printf("pick.Int64OrDefault(\"9999999999\", 0) → %d, ok=%v\n", i64, i64ok)

	uv, uok := pick.UintOrDefault("10", 0)
	fmt.Printf("pick.UintOrDefault(\"10\", 0) → %d, ok=%v\n", uv, uok)

	u32, u32ok := pick.Uint32OrDefault("255", 0)
	fmt.Printf("pick.Uint32OrDefault(\"255\", 0) → %d, ok=%v\n", u32, u32ok)

	u64, u64ok := pick.Uint64OrDefault("18446744073709551615", 0)
	fmt.Printf("pick.Uint64OrDefault(\"18446744073709551615\", 0) → %d, ok=%v\n", u64, u64ok)

	f32, f32ok := pick.Float32OrDefault("3.14", 0)
	fmt.Printf("pick.Float32OrDefault(\"3.14\", 0) → %.2f, ok=%v\n", f32, f32ok)

	f64, f64ok := pick.Float64OrDefault("2.71828", 0)
	fmt.Printf("pick.Float64OrDefault(\"2.71828\", 0) → %.5f, ok=%v\n", f64, f64ok)

	bv, bok := pick.BoolOrDefault("true", false)
	fmt.Printf("pick.BoolOrDefault(\"true\", false)  → %v, ok=%v\n", bv, bok)

	bv, bok = pick.BoolOrDefault("nope", false)
	fmt.Printf("pick.BoolOrDefault(\"nope\", false)  → %v, ok=%v\n", bv, bok)

	fmt.Println()
	fmt.Println("Done!")
}
