package compiler

import "testing"

func testSymbols() *SymbolTable {
	classes := map[string]*class{
		"Dog":          {name: "Dog", visibility: "private"},   // unexported
		"Animal":       {name: "Animal", visibility: "public"}, // exported
		"Box":          {name: "Box", visibility: "private"},   // unexported
		"Outer_Secret": {name: "Outer_Secret", visibility: "private"},
	}
	return buildSymbols(classes, nil, nil)
}

func TestResolveTypeExprRenamesPackagePrivate(t *testing.T) {
	syms := testSymbols()
	cases := map[string]string{
		"Dog":            "dog",
		"*Dog":           "*dog",
		"[]Dog":          "[]dog",
		"map[string]Dog": "map[string]dog",
		"Box[Dog]":       "box[dog]",
		"Animal":         "Animal", // public stays exported
		"int":            "int",    // builtin untouched
		"fmt.Stringer":   "fmt.Stringer",
		"Outer.Secret":   "outer_Secret", // nested private via dotted form
		"chan Dog":       "chan dog",
	}
	for in, want := range cases {
		if got := resolveTypeExpr(in, syms); got != want {
			t.Errorf("resolveTypeExpr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveSignatureResolvesParamsAndResults(t *testing.T) {
	syms := testSymbols()
	params, returns := resolveSignature("a int, b Dog", "Box[Dog]", syms)
	if params != "a int, b dog" {
		t.Errorf("params = %q", params)
	}
	if returns != "box[dog]" {
		t.Errorf("returns = %q", returns)
	}

	params2, returns2 := resolveSignature("", "(Dog, error)", syms)
	if params2 != "" || returns2 != "(dog, error)" {
		t.Errorf("empty/tuple = %q / %q", params2, returns2)
	}
}

func TestSymbolTableLookup(t *testing.T) {
	syms := testSymbols()
	if sym, ok := syms.Lookup("Dog"); !ok || sym.GoName != "dog" || sym.Kind != "class" {
		t.Fatalf("Dog symbol wrong: %+v (ok=%v)", sym, ok)
	}
	if sym, ok := syms.Lookup("Animal"); !ok || sym.GoName != "Animal" {
		t.Fatalf("Animal symbol wrong: %+v (ok=%v)", sym, ok)
	}
	if _, ok := syms.Lookup("Missing"); ok {
		t.Fatal("unexpected symbol Missing")
	}
}
