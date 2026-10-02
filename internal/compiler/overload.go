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

// inferCtx carries the information needed to type-check argument expressions and
// resolve overloaded calls inside one function/method body. It is the semantic
// analysis context of the typed IR.
type inferCtx struct {
	syms  *SymbolTable
	self  *class            // enclosing class, or nil for a free function
	scope map[string]string // local identifier -> GoOOP type
	ctors map[string]*class // emitted base constructor name -> class
	err   error
}

func newInferCtx(syms *SymbolTable, self *class, params string) *inferCtx {
	ctx := &inferCtx{syms: syms, self: self, scope: paramScope(params), ctors: map[string]*class{}}
	for _, sym := range syms.byName {
		if sym.Kind == "class" {
			ctx.ctors[ctorName(sym.Class)] = sym.Class
		}
	}
	return ctx
}

// paramScope maps parameter names to their declared types.
func paramScope(params string) map[string]string {
	scope := map[string]string{}
	ft, err := goparser.ParseExpr("func(" + params + ") {}")
	if err != nil {
		return scope
	}
	fn, ok := ft.(*ast.FuncLit)
	if !ok || fn.Type.Params == nil {
		return scope
	}
	for _, f := range fn.Type.Params.List {
		typ := printType(f.Type)
		for _, n := range f.Names {
			scope[n.Name] = typ
		}
	}
	return scope
}

// classByType resolves a GoOOP type string to its class declaration, ignoring a
// leading pointer and any generic argument list.
func (ctx *inferCtx) classByType(t string) *class {
	t = strings.TrimSpace(t)
	t = strings.TrimPrefix(t, "*")
	if i := strings.IndexByte(t, '['); i >= 0 {
		t = t[:i]
	}
	if sym, ok := ctx.syms.Lookup(t); ok {
		return sym.Class
	}
	return nil
}

// overloadGroup returns the own-class method set named `name` from the nearest
// class (self first) that declares it. Overloads live in one class; an override
// shadows the parent declaration rather than forming an overload with it, which
// matches how the methods are emitted (grouped per class). An inherited,
// genuinely-overloaded method is found on the ancestor that declares it, whose
// mangled names are promoted through Go embedding.
func (ctx *inferCtx) overloadGroup(c *class, name string) []method {
	seen := map[string]bool{}
	var walk func(*class) []method
	walk = func(cl *class) []method {
		if cl == nil || seen[cl.name] {
			return nil
		}
		seen[cl.name] = true
		var own []method
		for _, m := range cl.methods {
			if m.name == name && !m.ctor {
				own = append(own, m)
			}
		}
		if len(own) > 0 {
			return own
		}
		for _, ref := range allParents(cl) {
			if g := walk(ctx.syms.classFor(ref.name)); g != nil {
				return g
			}
		}
		return nil
	}
	return walk(c)
}

func (ctx *inferCtx) fieldType(c *class, name string) string {
	seen := map[string]bool{}
	var walk func(*class) string
	walk = func(cl *class) string {
		if cl == nil || seen[cl.name] {
			return ""
		}
		seen[cl.name] = true
		for _, f := range cl.fields {
			if f.name == name {
				return f.typ
			}
		}
		for _, ref := range allParents(cl) {
			if t := walk(ctx.syms.classFor(ref.name)); t != "" {
				return t
			}
		}
		return ""
	}
	return walk(c)
}

// inferType returns the GoOOP type of an expression, or "" if it cannot be
// inferred. Untyped constants are marked "untyped.<kind>".
func (ctx *inferCtx) inferType(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.BasicLit:
		return basicLitType(x.Kind)
	case *ast.Ident:
		switch x.Name {
		case "true", "false":
			return "untyped.bool"
		case "nil":
			return "untyped.nil"
		case "self":
			if ctx.self != nil {
				return "*" + ctx.self.name
			}
		}
		return ctx.scope[x.Name]
	case *ast.ParenExpr:
		return ctx.inferType(x.X)
	case *ast.UnaryExpr:
		if x.Op == gotoken.AND {
			if inner := ctx.inferType(x.X); inner != "" {
				return "*" + inner
			}
			return ""
		}
		return ctx.inferType(x.X)
	case *ast.StarExpr:
		if inner := ctx.inferType(x.X); strings.HasPrefix(inner, "*") {
			return inner[1:]
		}
		return ""
	case *ast.CompositeLit:
		if x.Type != nil {
			return printType(x.Type)
		}
	case *ast.SelectorExpr:
		return ctx.inferSelector(x)
	case *ast.CallExpr:
		return ctx.inferCall(x)
	case *ast.BinaryExpr:
		lt, rt := ctx.inferType(x.X), ctx.inferType(x.Y)
		if lt == rt {
			return lt
		}
		if strings.HasPrefix(lt, "untyped.") {
			return rt
		}
		return lt
	}
	return ""
}

func (ctx *inferCtx) inferSelector(x *ast.SelectorExpr) string {
	var c *class
	if id, ok := x.X.(*ast.Ident); ok && id.Name == "self" {
		c = ctx.self
	} else {
		c = ctx.classByType(ctx.inferType(x.X))
	}
	if c == nil {
		return ""
	}
	return normalizeIfType(ctx.fieldType(c, x.Sel.Name))
}

func (ctx *inferCtx) inferCall(x *ast.CallExpr) string {
	switch fun := x.Fun.(type) {
	case *ast.Ident:
		if cl, ok := ctx.ctors[fun.Name]; ok {
			return "*" + cl.name
		}
	case *ast.SelectorExpr:
		var c *class
		if id, ok := fun.X.(*ast.Ident); ok && id.Name == "self" {
			c = ctx.self
		} else {
			c = ctx.classByType(ctx.inferType(fun.X))
		}
		if c == nil {
			return ""
		}
		members := ctx.overloadGroup(c, fun.Sel.Name)
		if len(members) == 0 {
			return ""
		}
		idx := 0
		if len(members) > 1 {
			args := ctx.inferArgs(x.Args)
			chosen, err := selectOverload(fun.Sel.Name, members, args)
			if err != nil {
				return ""
			}
			idx = chosen
		}
		return normalizeIfType(members[idx].returns)
	}
	return ""
}

func (ctx *inferCtx) inferArgs(args []ast.Expr) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = ctx.inferType(a)
	}
	return out
}

func normalizeIfType(t string) string {
	if strings.TrimSpace(t) == "" {
		return ""
	}
	return normalizeType(t)
}

// overloadMethodName is the generated Go name of one member of an overloaded
// method set (visibility-correct base + signature suffix).
func overloadMethodName(m method) string {
	base := m.name
	if m.mods["public"] {
		base = exported(base)
	}
	suf, _ := overloadSuffix(m.params)
	return base + "__" + suf
}

// ctorOverloadName is the generated Go name of one member of an overloaded
// constructor set.
func ctorOverloadName(c *class, ctor method) string {
	suf, _ := overloadSuffix(ctor.params)
	return ctorName(c) + "__" + suf
}

// zeroCtorEmitName returns the emitted Go name of a class's zero-argument
// constructor (used when a subclass does not explicitly call super).
func zeroCtorEmitName(c *class) (string, error) {
	ctors := constructors(c)
	for _, ctor := range ctors {
		n, err := paramCount(ctor.params)
		if err != nil {
			return "", err
		}
		if n == 0 {
			if len(ctors) == 1 {
				return ctorName(c), nil
			}
			return ctorOverloadName(c, ctor), nil
		}
	}
	return "", fmt.Errorf("%s has no zero-argument constructor", c.name)
}

// resolveOverloadCalls performs semantic analysis over a parsed body: it builds
// local type bindings, then rewrites every overloaded method and constructor
// call to the specific generated name. Diagnostics (ambiguous, no-match,
// uninferrable) are surfaced as errors.
func (ctx *inferCtx) resolveOverloadCalls(root ast.Node) error {
	// Pass 1: collect local variable types in source order.
	ast.Inspect(root, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if x.Tok == gotoken.DEFINE {
				for i, lhs := range x.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok || id.Name == "_" || i >= len(x.Rhs) {
						continue
					}
					if t := ctx.inferType(x.Rhs[i]); t != "" {
						ctx.scope[id.Name] = t
					}
				}
			}
		case *ast.DeclStmt:
			gd, ok := x.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != gotoken.VAR {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				declared := ""
				if vs.Type != nil {
					declared = printType(vs.Type)
				}
				for i, nm := range vs.Names {
					if declared != "" {
						ctx.scope[nm.Name] = declared
					} else if i < len(vs.Values) {
						ctx.scope[nm.Name] = ctx.inferType(vs.Values[i])
					}
				}
			}
		}
		return true
	})
	// Pass 2: rewrite overloaded calls.
	ast.Inspect(root, func(n ast.Node) bool {
		if ctx.err != nil {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			ctx.rewriteCall(call)
		}
		return true
	})
	return ctx.err
}

func (ctx *inferCtx) fail(err error) {
	if ctx.err == nil {
		ctx.err = err
	}
}

func (ctx *inferCtx) rewriteCall(call *ast.CallExpr) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		var c *class
		if id, ok := fun.X.(*ast.Ident); ok && id.Name == "self" {
			c = ctx.self
		} else {
			c = ctx.classByType(ctx.inferType(fun.X))
		}
		if c == nil {
			return
		}
		members := ctx.overloadGroup(c, fun.Sel.Name)
		if len(members) < 2 {
			return // not overloaded
		}
		idx, err := selectOverload(fun.Sel.Name, members, ctx.inferArgs(call.Args))
		if err != nil {
			ctx.fail(err)
			return
		}
		fun.Sel = ast.NewIdent(overloadMethodName(members[idx]))
	case *ast.Ident:
		cl, ok := ctx.ctors[fun.Name]
		if !ok {
			return
		}
		ctors := constructors(cl)
		if len(ctors) < 2 {
			return
		}
		idx, err := selectOverload(cl.name, ctors, ctx.inferArgs(call.Args))
		if err != nil {
			ctx.fail(err)
			return
		}
		fun.Name = ctorOverloadName(cl, ctors[idx])
	}
}
