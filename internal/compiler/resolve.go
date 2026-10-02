package compiler

import (
	"go/ast"
	"go/format"
	goparser "go/parser"
	gotoken "go/token"
	"strings"
)

// resolveTypeExpr parses a Go type expression and rewrites references to
// GoOOP-declared types to their emitted Go names. Resolution is AST-based:
// identifiers are matched against the symbol table and rewritten structurally,
// never by textual substitution. Unrecognized or unparseable expressions are
// returned unchanged so ordinary Go types and external package types pass through.
func resolveTypeExpr(expr string, syms *SymbolTable) string {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return expr
	}
	node, err := goparser.ParseExpr(trimmed)
	if err != nil {
		return expr
	}
	resolved := resolveExpr(node, syms)
	var out strings.Builder
	if err := format.Node(&out, gotoken.NewFileSet(), resolved); err != nil {
		return expr
	}
	return out.String()
}

// resolveExpr walks a type expression and returns it with GoOOP type references
// resolved to their Go names. It handles the type-forming expression shapes
// (pointers, slices/arrays, maps, channels, generics, qualified nested classes).
func resolveExpr(e ast.Expr, syms *SymbolTable) ast.Expr {
	switch x := e.(type) {
	case *ast.Ident:
		if sym, ok := syms.Lookup(x.Name); ok {
			return ast.NewIdent(sym.GoName)
		}
		return x
	case *ast.SelectorExpr:
		// Outer.Inner refers to a flattened nested class Outer_Inner.
		if id, ok := x.X.(*ast.Ident); ok {
			if sym, ok := syms.Lookup(id.Name + "_" + x.Sel.Name); ok {
				return ast.NewIdent(sym.GoName)
			}
		}
		x.X = resolveExpr(x.X, syms)
		return x
	case *ast.StarExpr:
		x.X = resolveExpr(x.X, syms)
		return x
	case *ast.ParenExpr:
		x.X = resolveExpr(x.X, syms)
		return x
	case *ast.ArrayType:
		if x.Len != nil {
			x.Len = resolveExpr(x.Len, syms)
		}
		x.Elt = resolveExpr(x.Elt, syms)
		return x
	case *ast.Ellipsis:
		if x.Elt != nil {
			x.Elt = resolveExpr(x.Elt, syms)
		}
		return x
	case *ast.MapType:
		x.Key = resolveExpr(x.Key, syms)
		x.Value = resolveExpr(x.Value, syms)
		return x
	case *ast.ChanType:
		x.Value = resolveExpr(x.Value, syms)
		return x
	case *ast.IndexExpr:
		x.X = resolveExpr(x.X, syms)
		x.Index = resolveExpr(x.Index, syms)
		return x
	case *ast.IndexListExpr:
		x.X = resolveExpr(x.X, syms)
		for i := range x.Indices {
			x.Indices[i] = resolveExpr(x.Indices[i], syms)
		}
		return x
	case *ast.FuncType:
		resolveFieldList(x.Params, syms)
		resolveFieldList(x.Results, syms)
		return x
	}
	return e
}

func resolveFieldList(fl *ast.FieldList, syms *SymbolTable) {
	if fl == nil {
		return
	}
	for _, f := range fl.List {
		f.Type = resolveExpr(f.Type, syms)
	}
}

// resolveSignature resolves type references in a parameter list and a result
// list, returning both re-rendered. Parameter and result names are preserved.
func resolveSignature(params, returns string, syms *SymbolTable) (string, string) {
	ft, err := goparser.ParseExpr("func(" + params + ") " + strings.TrimSpace(returns))
	if err != nil {
		return params, returns
	}
	fn, ok := ft.(*ast.FuncType)
	if !ok {
		return params, returns
	}
	resolveFieldList(fn.Params, syms)
	resolveFieldList(fn.Results, syms)
	return printFieldList(fn.Params), printResults(fn.Results)
}

func printType(e ast.Expr) string {
	var b strings.Builder
	if err := format.Node(&b, gotoken.NewFileSet(), e); err != nil {
		return ""
	}
	return b.String()
}

func printFieldList(fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	var parts []string
	for _, f := range fl.List {
		typ := printType(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typ)
			continue
		}
		names := make([]string, len(f.Names))
		for i, n := range f.Names {
			names[i] = n.Name
		}
		parts = append(parts, strings.Join(names, ", ")+" "+typ)
	}
	return strings.Join(parts, ", ")
}

// metaRenameMap maps the exported metadata variable name of each unexported
// class to its unexported form, so references in bodies stay consistent.
func metaRenameMap(syms *SymbolTable) map[string]string {
	m := map[string]string{}
	for _, sym := range syms.byName {
		if sym.Kind == "class" && sym.GoName != sym.Name {
			m["GoOOPMetadata"+sym.Name] = "goOOPMetadata" + sym.Name
		}
	}
	return m
}

// resolveNode rewrites type references and metadata identifiers inside a parsed
// Go AST. Type references are only touched in genuine type positions (var/const
// specs, composite-literal and type-assertion types, field types), so value
// identifiers are never misinterpreted as types.
func resolveNode(n ast.Node, syms *SymbolTable) {
	meta := metaRenameMap(syms)
	ast.Inspect(n, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.ValueSpec:
			if x.Type != nil {
				x.Type = resolveExpr(x.Type, syms)
			}
		case *ast.CompositeLit:
			if x.Type != nil {
				x.Type = resolveExpr(x.Type, syms)
			}
		case *ast.TypeAssertExpr:
			if x.Type != nil {
				x.Type = resolveExpr(x.Type, syms)
			}
		case *ast.Field:
			if x.Type != nil {
				x.Type = resolveExpr(x.Type, syms)
			}
		case *ast.Ident:
			if r, ok := meta[x.Name]; ok {
				x.Name = r
			}
		}
		return true
	})
}

// resolveStmts resolves overloaded calls, type references and metadata
// identifiers in a statement list (a method or constructor body). Overload
// diagnostics are returned as errors; parse failures leave the body untouched.
func resolveStmts(body string, ctx *inferCtx) (string, error) {
	fset := gotoken.NewFileSet()
	file, err := goparser.ParseFile(fset, "body.go", "package p\nfunc _() {\n"+body+"\n}\n", goparser.ParseComments)
	if err != nil {
		return body, nil
	}
	if err := ctx.resolveOverloadCalls(file); err != nil {
		return "", err
	}
	resolveNode(file, ctx.syms)
	var out strings.Builder
	if err := format.Node(&out, fset, file); err != nil {
		return body, nil
	}
	s := out.String()
	open := strings.Index(s, "{")
	closeAt := strings.LastIndex(s, "}")
	if open < 0 || closeAt <= open {
		return body, nil
	}
	return strings.TrimSpace(s[open+1 : closeAt]), nil
}

// resolveFunc resolves a complete top-level function declaration.
func resolveFunc(fn string, ctx *inferCtx) (string, error) {
	fset := gotoken.NewFileSet()
	file, err := goparser.ParseFile(fset, "fn.go", "package p\n"+fn+"\n", goparser.ParseComments)
	if err != nil {
		return fn, nil
	}
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Type.Params != nil {
			for _, f := range fd.Type.Params.List {
				typ := printType(f.Type)
				for _, n := range f.Names {
					ctx.scope[n.Name] = typ
				}
			}
		}
	}
	if err := ctx.resolveOverloadCalls(file); err != nil {
		return "", err
	}
	resolveNode(file, ctx.syms)
	var out strings.Builder
	if err := format.Node(&out, fset, file); err != nil {
		return fn, nil
	}
	s := out.String()
	if idx := strings.Index(s, "func "); idx >= 0 {
		return strings.TrimSpace(s[idx:]), nil
	}
	return fn, nil
}

func printResults(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	inner := printFieldList(fl)
	if len(fl.List) == 1 && len(fl.List[0].Names) == 0 {
		return inner
	}
	return "(" + inner + ")"
}
