package compiler

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"strings"
)

// This file implements compile-time overload resolution for the typed IR:
// signature-based Go name mangling, an AST-based type inferencer for argument
// expressions, and overload matching by parameter/argument types. Nothing here
// uses textual heuristics — every type is parsed to an AST and resolved through
// the symbol table.

// mangleTypeExpr turns a Go type expression into a collision-safe identifier
// fragment used to name an overload, e.g. "*Dog" -> "ptrDog", "[]int" -> "sliceint".
func mangleTypeExpr(expr string) string {
	node, err := goparser.ParseExpr(strings.TrimSpace(expr))
	if err != nil {
		return sanitizeIdent(expr)
	}
	return mangleAST(node)
}

func mangleAST(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return "ptr" + mangleAST(x.X)
	case *ast.ArrayType:
		if x.Len == nil {
			return "slice" + mangleAST(x.Elt)
		}
		return "arr" + mangleAST(x.Elt)
	case *ast.MapType:
		return "map" + mangleAST(x.Key) + mangleAST(x.Value)
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return id.Name + "_" + x.Sel.Name
		}
		return mangleAST(x.X) + "_" + x.Sel.Name
	case *ast.IndexExpr:
		return mangleAST(x.X) + "_" + mangleAST(x.Index)
	case *ast.IndexListExpr:
		s := mangleAST(x.X)
		for _, idx := range x.Indices {
			s += "_" + mangleAST(idx)
		}
		return s
	case *ast.InterfaceType:
		return "any"
	case *ast.Ellipsis:
		return "vararg" + mangleAST(x.Elt)
	case *ast.ChanType:
		return "chan" + mangleAST(x.Value)
	case *ast.ParenExpr:
		return mangleAST(x.X)
	}
	return "T"
}

func sanitizeIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "T"
	}
	return b.String()
}

// overloadSuffix returns the signature fragment for a parameter list:
// "<arity>_<mangled types>". Distinct parameter types yield distinct suffixes,
// so overloads of equal arity never collide.
func overloadSuffix(params string) (string, error) {
	details, err := parameterDetails(params)
	if err != nil {
		return "", err
	}
	parts := make([]string, len(details))
	for i, d := range details {
		parts[i] = mangleTypeExpr(d)
	}
	return fmt.Sprintf("%d", len(details)) + "_" + strings.Join(parts, "_"), nil
}

// normalizeType canonicalizes a type string via the AST so equivalent spellings
// compare equal.
func normalizeType(t string) string {
	node, err := goparser.ParseExpr(strings.TrimSpace(t))
	if err != nil {
		return strings.Join(strings.Fields(t), "")
	}
	return printType(node)
}

func isIntegerType(t string) bool {
	switch t {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "byte", "rune":
		return true
	}
	return false
}

func isFloatType(t string) bool { return t == "float32" || t == "float64" }

func isNilableType(t string) bool {
	return strings.HasPrefix(t, "*") || strings.HasPrefix(t, "[]") ||
		strings.HasPrefix(t, "map[") || strings.HasPrefix(t, "chan") ||
		strings.HasPrefix(t, "func") || t == "any" || t == "error" ||
		t == "interface{}"
}

// assignScore rates how well an argument type fits a parameter type:
// 2 = exact, 1 = assignable, -1 = incompatible. Higher is more specific.
func assignScore(arg, param string) int {
	p := normalizeType(param)
	switch arg {
	case "untyped.int":
		if p == "int" {
			return 2
		}
		if isIntegerType(p) || isFloatType(p) {
			return 1
		}
	case "untyped.float":
		if p == "float64" {
			return 2
		}
		if isFloatType(p) {
			return 1
		}
	case "untyped.string":
		if p == "string" {
			return 2
		}
	case "untyped.bool":
		if p == "bool" {
			return 2
		}
	case "untyped.rune":
		if p == "rune" || p == "int32" {
			return 2
		}
		if isIntegerType(p) {
			return 1
		}
	case "untyped.nil":
		if isNilableType(p) {
			return 1
		}
	default:
		if normalizeType(arg) == p {
			return 2
		}
	}
	if p == "any" || p == "interface{}" {
		return 1
	}
	return -1
}

// selectOverload chooses the member of an overload set matching the given
// argument types, or returns a diagnostic. Arity-unique matches resolve without
// consulting argument types (the fast path); equal-arity sets are resolved by
// specificity, and ties are reported as ambiguous.
func selectOverload(name string, members []method, argTypes []string) (int, error) {
	arity := len(argTypes)
	var cands []int
	for i, m := range members {
		pc, err := paramCount(m.params)
		if err != nil {
			return -1, err
		}
		if pc == arity {
			cands = append(cands, i)
		}
	}
	if len(cands) == 0 {
		return -1, fmt.Errorf("no overload of %s accepts %d argument(s)", name, arity)
	}
	if len(cands) == 1 {
		return cands[0], nil
	}
	for i, at := range argTypes {
		if at == "" {
			return -1, fmt.Errorf("cannot resolve overloaded call to %s: type of argument %d could not be inferred", name, i+1)
		}
	}
	bestScore := -1
	var best []int
	for _, ci := range cands {
		details, err := parameterDetails(members[ci].params)
		if err != nil {
			return -1, err
		}
		total, ok := 0, true
		for k, pt := range details {
			s := assignScore(argTypes[k], pt)
			if s < 0 {
				ok = false
				break
			}
			total += s
		}
		if !ok {
			continue
		}
		if total > bestScore {
			bestScore, best = total, []int{ci}
		} else if total == bestScore {
			best = append(best, ci)
		}
	}
	if len(best) == 0 {
		return -1, fmt.Errorf("no matching overload of %s for argument types (%s)", name, strings.Join(argTypes, ", "))
	}
	if len(best) > 1 {
		return -1, fmt.Errorf("ambiguous overloaded call to %s for argument types (%s)", name, strings.Join(argTypes, ", "))
	}
	return best[0], nil
}

// basicLitType maps a literal token to its untyped-constant marker.
func basicLitType(kind gotoken.Token) string {
	switch kind {
	case gotoken.INT:
		return "untyped.int"
	case gotoken.FLOAT:
		return "untyped.float"
	case gotoken.STRING:
		return "untyped.string"
	case gotoken.CHAR:
		return "untyped.rune"
	}
	return ""
}
