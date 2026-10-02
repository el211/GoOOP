package compiler

import "testing"

func TestMangleTypeExpr(t *testing.T) {
	cases := map[string]string{
		"int":            "int",
		"*Dog":           "ptrDog",
		"[]int":          "sliceint",
		"map[string]int": "mapstringint",
		"Box[int]":       "Box_int",
		"Outer.Inner":    "Outer_Inner",
		"any":            "any",
	}
	for in, want := range cases {
		if got := mangleTypeExpr(in); got != want {
			t.Errorf("mangleTypeExpr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOverloadSuffix(t *testing.T) {
	cases := map[string]string{
		"":              "0_",
		"a int":         "1_int",
		"a int, b int":  "2_int_int",
		"a string":      "1_string",
		"a int, s *Dog": "2_int_ptrDog",
	}
	for in, want := range cases {
		got, err := overloadSuffix(in)
		if err != nil {
			t.Fatalf("overloadSuffix(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("overloadSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSelectOverloadArityFastPath(t *testing.T) {
	members := []method{
		{name: "Add", params: "a int"},
		{name: "Add", params: "a int, b int"},
	}
	// Arity 1 uniquely selects the first; arg type need not be known.
	if idx, err := selectOverload("Add", members, []string{""}); err != nil || idx != 0 {
		t.Fatalf("arity-1 fast path: idx=%d err=%v", idx, err)
	}
	if idx, err := selectOverload("Add", members, []string{"untyped.int", "untyped.int"}); err != nil || idx != 1 {
		t.Fatalf("arity-2: idx=%d err=%v", idx, err)
	}
	if _, err := selectOverload("Add", members, []string{"a", "b", "c"}); err == nil {
		t.Fatal("expected no-overload error for arity 3")
	}
}

func TestSelectOverloadSameArityByType(t *testing.T) {
	members := []method{
		{name: "Put", params: "v int"},
		{name: "Put", params: "v string"},
		{name: "Put", params: "v *Dog"},
	}
	if idx, err := selectOverload("Put", members, []string{"untyped.int"}); err != nil || idx != 0 {
		t.Fatalf("int: idx=%d err=%v", idx, err)
	}
	if idx, err := selectOverload("Put", members, []string{"untyped.string"}); err != nil || idx != 1 {
		t.Fatalf("string: idx=%d err=%v", idx, err)
	}
	if idx, err := selectOverload("Put", members, []string{"*Dog"}); err != nil || idx != 2 {
		t.Fatalf("*Dog: idx=%d err=%v", idx, err)
	}
	// Unknown argument type among same-arity candidates -> explicit diagnostic.
	if _, err := selectOverload("Put", members, []string{""}); err == nil {
		t.Fatal("expected uninferrable-argument diagnostic")
	}
	// No compatible candidate.
	if _, err := selectOverload("Put", members, []string{"untyped.bool"}); err == nil {
		t.Fatal("expected no-matching-overload diagnostic")
	}
}

func TestSelectOverloadAmbiguous(t *testing.T) {
	// nil is assignable to both pointer parameters with equal specificity.
	members := []method{
		{name: "Set", params: "v *Dog"},
		{name: "Set", params: "v *Cat"},
	}
	if _, err := selectOverload("Set", members, []string{"untyped.nil"}); err == nil {
		t.Fatal("expected ambiguous diagnostic for nil")
	}
}

func TestAssignScoreSpecificity(t *testing.T) {
	// exact beats interface fallback
	if s := assignScore("int", "int"); s != 2 {
		t.Errorf("int/int score=%d", s)
	}
	if s := assignScore("int", "any"); s != 1 {
		t.Errorf("int/any score=%d", s)
	}
	if s := assignScore("untyped.string", "int"); s != -1 {
		t.Errorf("string/int score=%d", s)
	}
}
