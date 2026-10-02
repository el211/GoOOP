// Package compiler translates GoOOP (.goop) source files into idiomatic Go.
// GoOOP is a source-to-source compiler; it does not modify the Go toolchain.
package compiler

import (
	"fmt"
	"go/ast"
	"go/format"
	goparser "go/parser"
	gotoken "go/token"
	"path/filepath"
	"sort"
	"strings"
)

type token struct {
	value      string
	start, end int
}
type method struct {
	name, params, returns, body string
	annotations                 []string
	mods                        map[string]bool
	hasBody, ctor               bool
}
type field struct {
	name, typ, initializer string
	annotations            []string
	mods                   map[string]bool
}
type parentRef struct{ name, args string }
type class struct {
	name, parent, parentArgs, typeParams, typeArgs string
	otherParents                                   []parentRef
	annotations                                    []string
	abstract                                       bool
	final                                          bool
	interfaces                                     []string
	fields                                         []field
	methods                                        []method
	nested                                         []*class
	// visibility is "public", "protected", "private" or "" (package-private).
	visibility string
	// enclosing is the top-level class that lexically contains this class, or
	// "" for a top-level class. Used for Java `private` nested-class scoping.
	enclosing string
	// pos is the source byte offset of the class name, for diagnostics.
	pos int
}
type iface struct {
	name, typeParams string
	annotations      []string
	methods          []method
}
type enumDecl struct {
	name        string
	members     []string
	annotations []string
}
type unit struct {
	header     string
	classes    []*class
	interfaces []iface
	enums      []enumDecl
	functions  []string
	// nestedDotted maps an outer class name to the set of its nested class
	// simple names, so `Outer.Inner` can be rewritten to `Outer_Inner`.
	nestedDotted map[string]map[string]bool
}
type parser struct {
	src  string
	toks []token
	at   int
}

// Compile translates one source. For inheritance across files use CompileFiles,
// which validates all declarations in a package together.
func Compile(filename string, src []byte) ([]byte, error) {
	results, err := CompileFiles(map[string][]byte{filename: src})
	if err != nil {
		return nil, err
	}
	return results[filename], nil
}

// CompileFiles resolves same-package declarations across source files.
// Files in different Go directories/packages are separate compilation units.
func CompileFiles(sources map[string][]byte) (map[string][]byte, error) {
	packages := map[string]map[string][]byte{}
	for filename, source := range sources {
		dir := filepath.Clean(filepath.Dir(filename))
		if packages[dir] == nil {
			packages[dir] = map[string][]byte{}
		}
		packages[dir][filename] = source
	}
	results := map[string][]byte{}
	for _, group := range packages {
		files := make([]string, 0, len(group))
		for name := range group {
			files = append(files, name)
		}
		sort.Strings(files)
		combined := &unit{}
		units := map[string]*unit{}
		classMap := map[string]*class{}
		for _, filename := range files {
			source := string(group[filename])
			p := &parser{src: source, toks: lex(source)}
			u, err := p.parse()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", filename, err)
			}
			units[filename] = u
			combined.classes = append(combined.classes, u.classes...)
			combined.interfaces = append(combined.interfaces, u.interfaces...)
			combined.functions = append(combined.functions, u.functions...)
			for _, c := range u.classes {
				classMap[c.name] = c
			}
		}
		if err := validate(combined); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Dir(files[0]), err)
		}
		for _, filename := range files {
			out, err := emit(units[filename], classMap)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", filename, err)
			}
			formatted, err := format.Source([]byte(out))
			if err != nil {
				return nil, fmt.Errorf("%s: invalid generated Go: %w\n--- generated source ---\n%s", filename, err, out)
			}
			results[filename] = formatted
		}
	}
	return results, nil
}

// lex preserves byte offsets, allowing method bodies and type expressions to
// remain native Go without round-tripping them through a custom expression AST.
func lex(s string) []token {
	var ts []token
	for i := 0; i < len(s); {
		c := s[i]
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			i++
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			i += 2
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && (s[i] != '*' || s[i+1] != '/') {
				i++
			}
			if i+1 < len(s) {
				i += 2
			}
			continue
		}
		begin := i
		if isIdentStart(c) {
			i++
			for i < len(s) && isIdentPart(s[i]) {
				i++
			}
		} else if c == '"' || c == '\'' || c == '`' {
			quote := c
			i++
			for i < len(s) {
				if s[i] == quote {
					i++
					break
				}
				if quote != '`' && s[i] == '\\' && i+1 < len(s) {
					i += 2
				} else {
					i++
				}
			}
		} else if c >= '0' && c <= '9' {
			i++
			for i < len(s) && (isIdentPart(s[i]) || s[i] == '.') {
				i++
			}
		} else {
			i++
		}
		ts = append(ts, token{s[begin:i], begin, i})
	}
	return ts
}
func isIdentStart(c byte) bool       { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isIdentPart(c byte) bool        { return isIdentStart(c) || c >= '0' && c <= '9' }
func (p *parser) peek(s string) bool { return p.at < len(p.toks) && p.toks[p.at].value == s }
func (p *parser) fail(at int, msg string) error {
	if at >= len(p.toks) {
		return fmt.Errorf("end of file: %s", msg)
	}
	line := strings.Count(p.src[:p.toks[at].start], "\n") + 1
	return fmt.Errorf("line %d near %q: %s", line, p.toks[at].value, msg)
}
func (p *parser) take() token { t := p.toks[p.at]; p.at++; return t }
func (p *parser) expect(v string) (token, error) {
	if !p.peek(v) {
		return token{}, p.fail(p.at, "expected "+v)
	}
	return p.take(), nil
}
func (p *parser) balanced(open, close string) (start, end token, err error) {
	start, err = p.expect(open)
	if err != nil {
		return
	}
	depth := 1
	for p.at < len(p.toks) {
		t := p.take()
		if t.value == open {
			depth++
		}
		if t.value == close {
			depth--
			if depth == 0 {
				return start, t, nil
			}
		}
	}
	err = p.fail(p.at, "unclosed "+open)
	return
}
func (p *parser) parse() (*unit, error) {
	u := &unit{}
	first := 0
	for first < len(p.toks) && p.toks[first].value != "class" && p.toks[first].value != "interface" && p.toks[first].value != "abstract" && p.toks[first].value != "func" && p.toks[first].value != "enum" && p.toks[first].value != "@" {
		first++
	}
	if first == len(p.toks) {
		return nil, fmt.Errorf("no class, interface or function declarations")
	}
	for first > 0 && isClassModifier(p.toks[first-1].value) {
		first--
	}
	u.header = strings.TrimSpace(p.src[:p.toks[first].start])
	if !strings.Contains(u.header, "package ") {
		u.header = "package main\n\n" + u.header
	}
	p.at = first
	for p.at < len(p.toks) {
		annotations, err := p.parseAnnotations()
		if err != nil {
			return nil, err
		}
		kind := ""
		for k := p.at; k < len(p.toks); k++ {
			if !isClassModifier(p.toks[k].value) {
				kind = p.toks[k].value
				break
			}
		}
		switch {
		case kind == "class":
			c, err := p.parseClass()
			if err != nil {
				return nil, err
			}
			if c.visibility == "private" || c.visibility == "protected" {
				return nil, fmt.Errorf("top-level class %s cannot be %s (only public or package-private)", c.name, c.visibility)
			}
			c.annotations = annotations
			u.classes = append(u.classes, c)
			hoistNested(u, c, c.name)
		case p.peek("interface"):
			in, err := p.parseInterface()
			if err != nil {
				return nil, err
			}
			in.annotations = annotations
			u.interfaces = append(u.interfaces, in)
		case p.peek("enum"):
			e, err := p.parseEnum()
			if err != nil {
				return nil, err
			}
			e.annotations = annotations
			u.enums = append(u.enums, e)
		case p.peek("func"):
			if len(annotations) > 0 {
				return nil, p.fail(p.at, "annotations on free functions not supported")
			}
			start := p.take()
			for p.at < len(p.toks) && !p.peek("{") {
				p.at++
			}
			if p.at == len(p.toks) {
				return nil, p.fail(p.at, "function requires a body")
			}
			_, end, err := p.balanced("{", "}")
			if err != nil {
				return nil, err
			}
			u.functions = append(u.functions, p.src[start.start:end.end])
		default:
			return nil, p.fail(p.at, "unsupported top-level declaration")
		}
	}
	return u, nil
}
func isClassModifier(v string) bool {
	switch v {
	case "public", "private", "protected", "abstract", "final":
		return true
	}
	return false
}

// hoistNested flattens nested classes into top-level classes named
// `Outer_Inner`, recording the dotted mapping so `Outer.Inner` can be rewritten.
// topLevel is the outermost enclosing class, used for `private` nested scoping.
func hoistNested(u *unit, c *class, topLevel string) {
	for _, n := range c.nested {
		if u.nestedDotted == nil {
			u.nestedDotted = map[string]map[string]bool{}
		}
		if u.nestedDotted[c.name] == nil {
			u.nestedDotted[c.name] = map[string]bool{}
		}
		u.nestedDotted[c.name][n.name] = true
		n.name = c.name + "_" + n.name
		n.enclosing = topLevel
		u.classes = append(u.classes, n)
		hoistNested(u, n, topLevel)
	}
}
func (p *parser) parseClass() (*class, error) {
	c := &class{}
	for p.peek("abstract") || p.peek("final") || p.peek("public") || p.peek("private") || p.peek("protected") {
		switch tok := p.take().value; tok {
		case "abstract":
			c.abstract = true
		case "final":
			c.final = true
		default: // visibility
			if c.visibility != "" {
				return nil, p.fail(p.at, "class has more than one visibility modifier")
			}
			c.visibility = tok
		}
	}
	if c.abstract && c.final {
		return nil, p.fail(p.at, "class cannot be both abstract and final")
	}
	if _, err := p.expect("class"); err != nil {
		return nil, err
	}
	if p.at == len(p.toks) {
		return nil, p.fail(p.at, "missing class name")
	}
	nameTok := p.take()
	c.name = nameTok.value
	c.pos = nameTok.start
	if p.peek("[") {
		open, close, err := p.balanced("[", "]")
		if err != nil {
			return nil, err
		}
		c.typeParams = strings.TrimSpace(p.src[open.end:close.start])
		c.typeArgs, err = typeParamArgs(c.typeParams)
		if err != nil {
			return nil, fmt.Errorf("class %s: %w", c.name, err)
		}
	}
	if p.peek("extends") {
		p.take()
		if p.at == len(p.toks) {
			return nil, p.fail(p.at, "missing superclass")
		}
		c.parent = p.take().value
		if p.peek("[") {
			open, close, err := p.balanced("[", "]")
			if err != nil {
				return nil, err
			}
			c.parentArgs = p.src[open.start:close.end]
		}
		seenParents := map[string]bool{c.parent: true}
		for p.peek(",") {
			p.take()
			if p.at == len(p.toks) {
				return nil, p.fail(p.at, "missing additional superclass")
			}
			next := parentRef{name: p.take().value}
			if seenParents[next.name] {
				return nil, p.fail(p.at, "duplicate superclass "+next.name)
			}
			seenParents[next.name] = true
			if p.peek("[") {
				open, close, err := p.balanced("[", "]")
				if err != nil {
					return nil, err
				}
				next.args = p.src[open.start:close.end]
			}
			c.otherParents = append(c.otherParents, next)
		}
	}
	if p.peek("implements") {
		p.take()
		for {
			if p.at == len(p.toks) {
				return nil, p.fail(p.at, "missing interface")
			}
			in := p.take().value
			if p.peek("[") {
				open, close, err := p.balanced("[", "]")
				if err != nil {
					return nil, err
				}
				in += p.src[open.start:close.end]
			}
			c.interfaces = append(c.interfaces, in)
			if !p.peek(",") {
				break
			}
			p.take()
		}
	}
	if _, err := p.expect("{"); err != nil {
		return nil, err
	}
	for p.at < len(p.toks) && !p.peek("}") {
		annotations, err := p.parseAnnotations()
		if err != nil {
			return nil, err
		}
		mods := p.modifiers()
		if p.peek("}") {
			break
		}
		if p.at == len(p.toks) {
			break
		}
		if p.peek("class") {
			nc, err := p.parseClass() // modifiers already consumed any abstract/final/visibility
			if err != nil {
				return nil, err
			}
			nc.abstract = nc.abstract || mods["abstract"]
			nc.final = nc.final || mods["final"]
			vis := []string{}
			for _, v := range []string{"public", "protected", "private"} {
				if mods[v] {
					vis = append(vis, v)
				}
			}
			if len(vis) > 1 {
				return nil, p.fail(p.at, "nested class "+nc.name+" has more than one visibility modifier")
			}
			if len(vis) == 1 {
				nc.visibility = vis[0]
			}
			nc.annotations = annotations
			c.nested = append(c.nested, nc)
			continue
		}
		isProperty := false
		if p.peek("property") {
			p.take()
			isProperty = true
			if mods["static"] {
				return nil, p.fail(p.at, "property cannot be static")
			}
		}
		name := p.take()
		if p.peek("(") {
			if isProperty {
				return nil, p.fail(p.at, "property must declare a field, not a method")
			}
			m, err := p.parseMethod(name.value, mods)
			if err != nil {
				return nil, err
			}
			m.annotations = annotations
			m.ctor = m.name == "constructor"
			if m.ctor && len(strings.TrimSpace(m.returns)) > 0 {
				return nil, p.fail(p.at, "constructor cannot have a return type")
			}
			if !m.hasBody && !mods["abstract"] {
				return nil, p.fail(p.at, "body-less method must be abstract")
			}
			c.methods = append(c.methods, m)
		} else {
			f, err := p.parseField(name, mods)
			if err != nil {
				return nil, err
			}
			f.annotations = annotations
			if isProperty {
				f.mods["property"] = true
			}
			c.fields = append(c.fields, f)
		}
	}
	if _, err := p.expect("}"); err != nil {
		return nil, err
	}
	return c, nil
}

// parseAnnotations accepts @Name and @Name(native Go argument expressions).
// Annotations are declaration metadata, not executable decorators.
func (p *parser) parseAnnotations() ([]string, error) {
	var result []string
	for p.peek("@") {
		p.take()
		if p.at == len(p.toks) || !isIdentStart(p.toks[p.at].value[0]) {
			return nil, p.fail(p.at, "expected annotation name")
		}
		name := p.take().value
		if p.peek("(") {
			open, close, err := p.balanced("(", ")")
			if err != nil {
				return nil, err
			}
			name += p.src[open.start:close.end]
		}
		result = append(result, name)
	}
	return result, nil
}

// typeParamArgs turns "T any, K comparable" into "T, K".
func typeParamArgs(params string) (string, error) {
	src := "package p\ntype X[" + params + "] struct{}"
	file, err := goparser.ParseFile(gotoken.NewFileSet(), "generic.go", src, 0)
	if err != nil {
		return "", err
	}
	t := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	var names []string
	for _, field := range t.TypeParams.List {
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("empty type parameter list")
	}
	return strings.Join(names, ", "), nil
}

func (p *parser) modifiers() map[string]bool {
	m := map[string]bool{}
	for p.peek("public") || p.peek("private") || p.peek("protected") || p.peek("abstract") || p.peek("virtual") || p.peek("override") || p.peek("static") || p.peek("final") {
		m[p.take().value] = true
	}
	return m
}
func (p *parser) parseField(name token, mods map[string]bool) (field, error) {
	start := p.at
	round, square, curly := 0, 0, 0
	for p.at < len(p.toks) {
		next := p.toks[p.at]
		if round == 0 && square == 0 && curly == 0 {
			if next.value == "}" || next.value == ";" {
				break
			}
			previous := name
			if p.at > start {
				previous = p.toks[p.at-1]
			}
			if strings.Contains(p.src[previous.end:next.start], "\n") {
				break
			}
		}
		switch next.value {
		case "(":
			round++
		case ")":
			round--
		case "[":
			square++
		case "]":
			square--
		case "{":
			curly++
		case "}":
			curly--
		}
		p.at++
	}
	if start == p.at {
		return field{}, p.fail(start, "field requires a Go type")
	}
	typ := strings.TrimSpace(p.src[p.toks[start].start:p.toks[p.at-1].end])
	if p.peek(";") {
		p.take()
	}
	initializer := ""
	fieldTokens := lex(typ)
	for _, tok := range fieldTokens {
		if tok.value == "=" {
			initializer = strings.TrimSpace(typ[tok.end:])
			typ = strings.TrimSpace(typ[:tok.start])
			if initializer == "" {
				return field{}, p.fail(start, "missing field initializer")
			}
			break
		}
	}
	return field{name: name.value, typ: typ, initializer: initializer, mods: mods}, nil
}
func (p *parser) parseMethod(name string, mods map[string]bool) (method, error) {
	open, close, err := p.balanced("(", ")")
	if err != nil {
		return method{}, err
	}
	m := method{name: name, params: strings.TrimSpace(p.src[open.end:close.start]), mods: mods}
	retStart := p.at
	for p.at < len(p.toks) && !p.peek("{") && !p.peek(";") && !p.peek("}") {
		if p.at > retStart && strings.Contains(p.src[p.toks[p.at-1].end:p.toks[p.at].start], "\n") {
			break
		}
		p.at++
	}
	if p.at > retStart {
		m.returns = strings.TrimSpace(p.src[p.toks[retStart].start:p.toks[p.at-1].end])
	}
	if p.peek("{") {
		bopen, bclose, err := p.balanced("{", "}")
		if err != nil {
			return method{}, err
		}
		m.body = p.src[bopen.end:bclose.start]
		m.hasBody = true
	} else if p.peek(";") {
		p.take()
	}
	return m, nil
}

// parseEnum accepts `enum Name { A, B, C }`; members are identifiers separated
// by commas and/or newlines, with an optional trailing comma.
func (p *parser) parseEnum() (enumDecl, error) {
	p.take() // enum
	if p.at == len(p.toks) {
		return enumDecl{}, p.fail(p.at, "enum requires a name")
	}
	e := enumDecl{name: p.take().value}
	if _, err := p.expect("{"); err != nil {
		return enumDecl{}, err
	}
	seen := map[string]bool{}
	for p.at < len(p.toks) && !p.peek("}") {
		if p.peek(",") {
			p.take()
			continue
		}
		tok := p.take()
		if !isIdentStart(tok.value[0]) {
			return enumDecl{}, p.fail(p.at, "enum member must be an identifier")
		}
		if seen[tok.value] {
			return enumDecl{}, p.fail(p.at, "duplicate enum member "+tok.value)
		}
		seen[tok.value] = true
		e.members = append(e.members, tok.value)
	}
	if _, err := p.expect("}"); err != nil {
		return enumDecl{}, err
	}
	if len(e.members) == 0 {
		return enumDecl{}, p.fail(p.at, "enum requires at least one member")
	}
	return e, nil
}
func (p *parser) parseInterface() (iface, error) {
	p.take()
	if p.at == len(p.toks) {
		return iface{}, p.fail(p.at, "interface requires a name")
	}
	in := iface{name: p.take().value}
	if p.peek("[") {
		open, close, err := p.balanced("[", "]")
		if err != nil {
			return in, err
		}
		in.typeParams = strings.TrimSpace(p.src[open.end:close.start])
	}
	if _, err := p.expect("{"); err != nil {
		return in, err
	}
	for p.at < len(p.toks) && !p.peek("}") {
		anns, err := p.parseAnnotations()
		if err != nil {
			return in, err
		}
		mods := p.modifiers()
		if p.at == len(p.toks) {
			break
		}
		name := p.take()
		if !p.peek("(") {
			return in, p.fail(p.at, "interface members must be methods")
		}
		m, err := p.parseMethod(name.value, mods)
		if err != nil {
			return in, err
		}
		if m.hasBody {
			return in, p.fail(p.at, "interface method cannot have a body")
		}
		m.annotations = anns
		in.methods = append(in.methods, m)
	}
	if _, err := p.expect("}"); err != nil {
		return in, err
	}
	return in, nil
}
func allParents(c *class) []parentRef {
	if c == nil || c.parent == "" {
		return nil
	}
	return append([]parentRef{{name: c.parent, args: c.parentArgs}}, c.otherParents...)
}

func validate(u *unit) error {
	classes := map[string]*class{}
	interfaces := map[string]bool{}
	for _, in := range u.interfaces {
		if interfaces[in.name] {
			return fmt.Errorf("duplicate interface %s", in.name)
		}
		interfaces[in.name] = true
	}
	for _, c := range u.classes {
		if classes[c.name] != nil || interfaces[c.name] {
			return fmt.Errorf("duplicate class %s", c.name)
		}
		classes[c.name] = c
		constructorSigs := map[string]bool{}
		methodsByName := map[string][]method{}
		for _, m := range c.methods {
			if !m.ctor {
				methodsByName[m.name] = append(methodsByName[m.name], m)
			} else {
				sig, err := overloadSuffix(m.params)
				if err != nil {
					return fmt.Errorf("%s constructor: %w", c.name, err)
				}
				if constructorSigs[sig] {
					return fmt.Errorf("%s: duplicate constructor signature (%s)", c.name, strings.TrimSpace(m.params))
				}
				constructorSigs[sig] = true
				if !m.hasBody {
					return fmt.Errorf("%s: constructor needs a body", c.name)
				}
			}
			if m.mods["abstract"] && !c.abstract {
				return fmt.Errorf("%s.%s: abstract method requires abstract class", c.name, m.name)
			}
			if m.mods["static"] && (m.mods["abstract"] || m.mods["virtual"] || m.mods["override"] || m.ctor) {
				return fmt.Errorf("%s.%s: unsupported static modifier combination", c.name, m.name)
			}
			if m.mods["final"] && (m.mods["abstract"] || m.mods["static"] || m.ctor) {
				return fmt.Errorf("%s.%s: unsupported final modifier combination", c.name, m.name)
			}
		}
		for name, group := range methodsByName {
			if len(group) < 2 {
				continue
			}
			sigs := map[string]bool{}
			for _, member := range group {
				// Overloads are resolved statically, so they may differ by
				// parameter type and even return type, but must be concrete,
				// nonvirtual instance methods (virtual/interface overloads need
				// the dispatch work of a later milestone).
				if member.mods["abstract"] || member.mods["virtual"] || member.mods["override"] || member.mods["static"] {
					return fmt.Errorf("%s.%s: overloaded methods must be concrete, nonvirtual instance methods", c.name, name)
				}
				sig, err := overloadSuffix(member.params)
				if err != nil {
					return err
				}
				if sigs[sig] {
					return fmt.Errorf("%s.%s: duplicate overload signature (%s)", c.name, name, strings.TrimSpace(member.params))
				}
				sigs[sig] = true
			}
		}
	}
	syms := buildSymbols(classes, u.interfaces, u.enums)
	for _, c := range u.classes {
		// Traverse every edge, including additional embedded superclasses.
		var walkAncestors func(string, map[string]bool) error
		walkAncestors = func(name string, path map[string]bool) error {
			if path[name] {
				return fmt.Errorf("inheritance cycle involving %s", name)
			}
			parent := classes[name]
			if parent == nil {
				return fmt.Errorf("%s: superclass %s must be in the same Go package", c.name, name)
			}
			path[name] = true
			for _, ref := range allParents(parent) {
				if err := walkAncestors(ref.name, path); err != nil {
					return err
				}
			}
			delete(path, name)
			return nil
		}
		for _, ref := range allParents(c) {
			if err := walkAncestors(ref.name, map[string]bool{c.name: true}); err != nil {
				return err
			}
			if parent := classes[ref.name]; parent != nil && parent.final {
				return fmt.Errorf("%s: cannot extend final class %s", c.name, ref.name)
			}
		}
		// Go embedding cannot select an inherited method promoted by two direct
		// parents with the SAME signature. Distinct signatures are overloads and
		// merge; an identical signature from two parents is a real conflict that
		// demands an explicit child override. Keys are name#paramSignature.
		memberKey := func(m method) string {
			sig, _ := overloadSuffix(m.params)
			return m.name + "#" + sig
		}
		declared := map[string]bool{}
		for _, m := range c.methods {
			if !m.ctor && !m.mods["static"] {
				declared[memberKey(m)] = true
			}
		}
		promoted := map[string]string{}
		for _, ref := range allParents(c) {
			visited := map[string]bool{}
			var collect func(*class)
			collect = func(parent *class) {
				if parent == nil || visited[parent.name] {
					return
				}
				visited[parent.name] = true
				for _, m := range parent.methods {
					if m.ctor || m.mods["static"] || m.mods["private"] {
						continue
					}
					key := memberKey(m)
					if prior, ok := promoted[key]; ok && prior != ref.name && !declared[key] {
						promoted[key] = prior + "," + ref.name
					} else if !ok {
						promoted[key] = ref.name
					}
				}
				for _, next := range allParents(parent) {
					collect(classes[next.name])
				}
			}
			collect(classes[ref.name])
		}
		for key, origin := range promoted {
			if strings.Contains(origin, ",") && !declared[key] {
				name := key[:strings.IndexByte(key, '#')]
				return fmt.Errorf("%s: inherited method %s is ambiguous between %s; explicitly override it", c.name, name, origin)
			}
		}
		if !c.abstract {
			unresolved := map[string]bool{}
			seenLineage := map[string]bool{}
			var merge func(*class)
			merge = func(node *class) {
				if node == nil || seenLineage[node.name] {
					return
				}
				seenLineage[node.name] = true
				for _, ref := range allParents(node) {
					merge(classes[ref.name])
				}
				for _, m := range node.methods {
					if m.ctor || m.mods["static"] {
						continue
					}
					if m.mods["abstract"] {
						unresolved[m.name] = true
					} else {
						delete(unresolved, m.name)
					}
				}
			}
			merge(c)
			for name := range unresolved {
				return fmt.Errorf("%s: concrete class must implement abstract method %s", c.name, name)
			}
		}
		for _, m := range c.methods {
			if m.mods["override"] {
				found := false
				var check func(parentRef, map[string]string, map[string]bool) error
				check = func(ref parentRef, outer map[string]string, visited map[string]bool) error {
					parent := classes[ref.name]
					if parent == nil {
						return nil
					}
					// A parent may be reached through multiple independent
					// inheritance paths. Only suppress cycles on the active path.
					if visited[parent.name] {
						return nil
					}
					visited[parent.name] = true
					defer delete(visited, parent.name)
					binding := map[string]string{}
					variables := strings.Split(parent.typeArgs, ",")
					actual := splitTypeArguments(ref.args)
					for i, v := range variables {
						if i < len(actual) && strings.TrimSpace(v) != "" {
							binding[strings.TrimSpace(v)] = substituteNames(strings.TrimSpace(actual[i]), outer)
						}
					}
					for _, pm := range parent.methods {
						if pm.name == m.name && !pm.ctor && !pm.mods["static"] {
							if pm.mods["private"] {
								return fmt.Errorf("%s.%s: cannot override private superclass method", c.name, m.name)
							}
							if pm.mods["final"] {
								return fmt.Errorf("%s.%s: cannot override final superclass method", c.name, m.name)
							}
							found = true
							specialized := pm
							specialized.params = substituteNames(pm.params, binding)
							specialized.returns = substituteNames(pm.returns, binding)
							psig, _ := overloadSuffix(specialized.params)
							msig, _ := overloadSuffix(m.params)
							if psig != msig {
								return fmt.Errorf("%s.%s: override parameters differ from superclass %s", c.name, m.name, parent.name)
							}
							// Return types must be identical or covariant (the
							// override may return a subtype through the hierarchy).
							if normalizeType(m.returns) != normalizeType(specialized.returns) {
								if !syms.isSubtype(m.returns, specialized.returns) {
									return fmt.Errorf("%s.%s: override return type %q is not %q or a covariant subtype", c.name, m.name, strings.TrimSpace(m.returns), strings.TrimSpace(specialized.returns))
								}
								// Covariant returns are representable in Go only when
								// the overridden method is not virtually dispatched;
								// a virtual/abstract parent would need a dispatch
								// bridge (planned milestone).
								if pm.mods["virtual"] || pm.mods["abstract"] || pm.mods["override"] {
									return fmt.Errorf("%s.%s: covariant return type on a virtual/abstract override is not yet supported (parent %s.%s participates in virtual dispatch); use an identical return type, a non-virtual parent, or an interface", c.name, m.name, parent.name, pm.name)
								}
							}
						}
					}
					for _, next := range allParents(parent) {
						if err := check(next, binding, visited); err != nil {
							return err
						}
					}
					return nil
				}
				for _, ref := range allParents(c) {
					if err := check(ref, nil, map[string]bool{}); err != nil {
						return err
					}
				}
				if !found {
					return fmt.Errorf("%s.%s: override requires an inherited method", c.name, m.name)
				}
			}
			if !m.ctor && hasSuperConstructor(m.body) {
				return fmt.Errorf("%s.%s: super(...) only allowed in a constructor", c.name, m.name)
			}

		}
	}
	if err := checkVisibilityAccess(u, classes); err != nil {
		return err
	}
	return nil
}

// topLevelOf returns the outermost class enclosing c (c itself if top-level).
func topLevelOf(c *class) string {
	if c.enclosing != "" {
		return c.enclosing
	}
	return c.name
}

// classAccessible applies Java access rules within one package: a private class
// is reachable only from within its enclosing top-level class; everything else
// is reachable package-wide.
func classAccessible(ref *class, fromTopLevel string) bool {
	if ref.visibility != "private" {
		return true
	}
	return fromTopLevel != "" && ref.enclosing == fromTopLevel
}

// scanClassRefs reports a disallowed access to a private class if `src` contains
// a dotted reference `A.B` whose flattened class `A_B` is not accessible.
func scanClassRefs(src string, classes map[string]*class, fromTopLevel, where string) error {
	ts := lex(src)
	for i := 0; i+2 < len(ts); i++ {
		if ts[i+1].value != "." || !isIdentStart(ts[i].value[0]) || !isIdentStart(ts[i+2].value[0]) {
			continue
		}
		if ref := classes[ts[i].value+"_"+ts[i+2].value]; ref != nil && !classAccessible(ref, fromTopLevel) {
			return fmt.Errorf("%s: cannot access %s class %s.%s", where, ref.visibility, ts[i].value, ts[i+2].value)
		}
	}
	return nil
}

func checkVisibilityAccess(u *unit, classes map[string]*class) error {
	for _, c := range u.classes {
		top := topLevelOf(c)
		for _, ref := range allParents(c) {
			p := classes[ref.name]
			if p == nil {
				continue
			}
			if !classAccessible(p, top) {
				return fmt.Errorf("%s: cannot extend %s class %s", c.name, p.visibility, ref.name)
			}
		}
		for _, f := range c.fields {
			for _, s := range []string{f.typ, f.initializer} {
				if err := scanClassRefs(s, classes, top, c.name); err != nil {
					return err
				}
			}
		}
		for _, m := range c.methods {
			for _, s := range []string{m.params, m.returns, m.body} {
				if err := scanClassRefs(s, classes, top, c.name+"."+m.name); err != nil {
					return err
				}
			}
		}
	}
	for _, fn := range u.functions {
		if err := scanClassRefs(fn, classes, "", "function"); err != nil {
			return err
		}
	}
	return nil
}
func splitTypeArguments(bracket string) []string {
	if !strings.HasPrefix(bracket, "[") || !strings.HasSuffix(bracket, "]") {
		return nil
	}
	body := bracket[1 : len(bracket)-1]
	ts := lex(body)
	depth := 0
	cursor := 0
	var result []string
	for _, v := range ts {
		switch v.value {
		case "[", "(", "{":
			depth++
		case "]", ")", "}":
			depth--
		case ",":
			if depth == 0 {
				result = append(result, strings.TrimSpace(body[cursor:v.start]))
				cursor = v.end
			}
		}
	}
	result = append(result, strings.TrimSpace(body[cursor:]))
	return result
}
func substituteNames(source string, substitutions map[string]string) string {
	if len(substitutions) == 0 {
		return source
	}
	tokens := lex(source)
	var out strings.Builder
	cursor := 0
	for _, tok := range tokens {
		if replacement, ok := substitutions[tok.value]; ok {
			out.WriteString(source[cursor:tok.start])
			out.WriteString(replacement)
			cursor = tok.end
		}
	}
	out.WriteString(source[cursor:])
	return out.String()
}

func hasSuperConstructor(s string) bool {
	ts := lex(s)
	for i := 0; i+1 < len(ts); i++ {
		if ts[i].value == "super" && ts[i+1].value == "(" {
			return true
		}
		if i+3 < len(ts) && ts[i].value == "super" && ts[i+1].value == "." && ts[i+3].value == "(" {
			return true
		}
	}
	return false
}

// paramCount counts Go parameters, not comma-separated groups (a,b int is 2).
// parameterDetails expands grouped parameter declarations into Go types.
func parameterDetails(params string) ([]string, error) {
	source := "package p\nfunc f(" + params + ") {}"
	file, err := goparser.ParseFile(gotoken.NewFileSet(), "args.go", source, 0)
	if err != nil {
		return nil, err
	}
	fn := file.Decls[0].(*ast.FuncDecl)
	result := []string{}
	for _, field := range fn.Type.Params.List {
		var typeText strings.Builder
		if err := format.Node(&typeText, gotoken.NewFileSet(), field.Type); err != nil {
			return nil, err
		}
		typ := typeText.String()
		if strings.HasPrefix(typ, "...") {
			return nil, fmt.Errorf("variadic overloads are not supported")
		}
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			result = append(result, typ)
		}
	}
	return result, nil
}

func paramCount(params string) (int, error) {
	src := "package p\nfunc f(" + params + ") {}"
	file, err := goparser.ParseFile(gotoken.NewFileSet(), "params.go", src, 0)
	if err != nil {
		return 0, err
	}
	fn := file.Decls[0].(*ast.FuncDecl)
	count := 0
	for _, field := range fn.Type.Params.List {
		n := len(field.Names)
		if n == 0 {
			n = 1
		}
		count += n
	}
	return count, nil
}

func constructors(c *class) []method {
	var out []method
	for _, m := range c.methods {
		if m.ctor {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		out = append(out, method{ctor: true, hasBody: true, mods: map[string]bool{}})
	}
	return out
}

// invocationCount counts top-level call arguments, skipping nested (), [] and {}.
func invocationCount(ts []token, open int) (int, int, error) {
	if open >= len(ts) || ts[open].value != "(" {
		return 0, 0, fmt.Errorf("expected invocation")
	}
	depth, square, curly := 1, 0, 0
	n := 0
	started := false
	for j := open + 1; j < len(ts); j++ {
		v := ts[j].value
		if v == ")" && depth == 1 && square == 0 && curly == 0 {
			if started {
				n++
			}
			return n, j, nil
		}
		if v == "," && depth == 1 && square == 0 && curly == 0 {
			n++
			started = false
			continue
		}
		started = true
		switch v {
		case "(":
			depth++
		case ")":
			depth--
		case "[":
			square++
		case "]":
			square--
		case "{":
			curly++
		case "}":
			curly--
		}
	}
	return 0, 0, fmt.Errorf("unterminated invocation")
}

// leadingSupers moves explicit parent constructors before field initialization.
// For multiple inheritance use super(args) for the first parent and
// super.OtherParent(args) for additional parents, all at the start of the body.
func leadingSupers(body string, c *class) (map[string]string, string, error) {
	ts := lex(body)
	calls := map[string]string{}
	cursor := 0
	for cursor < len(ts) && ts[cursor].value == "super" {
		from := cursor
		parent := c.parent
		open := cursor + 1
		if open < len(ts) && ts[open].value == "." {
			if open+1 >= len(ts) {
				return nil, "", fmt.Errorf("missing named parent after super.")
			}
			parent = ts[open+1].value
			open += 2
		}
		if open >= len(ts) || ts[open].value != "(" {
			break
		} // super.Method() is an ordinary method call
		exists := false
		for _, ref := range allParents(c) {
			if ref.name == parent {
				exists = true
			}
		}
		if !exists {
			return nil, "", fmt.Errorf("unknown superclass %s in super constructor", parent)
		}
		if _, seen := calls[parent]; seen {
			return nil, "", fmt.Errorf("superclass %s initialized twice", parent)
		}
		_, end, err := invocationCount(ts, open)
		if err != nil {
			return nil, "", err
		}
		calls[parent] = body[ts[from].start:ts[end].end]
		cursor = end + 1
		if cursor < len(ts) && ts[cursor].value == ";" {
			cursor++
		}
	}
	// A superclass constructor may not follow arbitrary statements.
	for i := cursor; i < len(ts); i++ {
		if ts[i].value == "super" && i+1 < len(ts) {
			next := ts[i+1].value
			if next == "(" || next == "." && i+3 < len(ts) && ts[i+3].value == "(" && (ts[i+2].value == c.parent || containsParent(c, ts[i+2].value)) {
				return nil, "", fmt.Errorf("superclass constructors must be called before other statements")
			}
		}
	}
	if cursor == 0 {
		return calls, body, nil
	}
	return calls, body[:ts[0].start] + body[ts[cursor-1].end:], nil
}
func containsParent(c *class, name string) bool {
	for _, ref := range allParents(c) {
		if ref.name == name {
			return true
		}
	}
	return false
}

func unexported(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// unexportedClass reports whether a class must be emitted with an unexported Go
// identifier. Following Java, `private` and package-private (default) classes are
// not visible outside their package, so they are unexported; `public` and
// `protected` classes stay exported (protected needs cross-package subclassing).
func unexportedClass(c *class) bool {
	return c.visibility != "public" && c.visibility != "protected"
}

// goTypeName is the Go identifier emitted for a class, keeping its visibility
// out of the generated public API.
func goTypeName(c *class) string {
	if unexportedClass(c) {
		return unexported(c.name)
	}
	return c.name
}

// goFieldName is the Go identifier used to embed/select a class by name,
// resolving to its (possibly unexported) emitted type name.
func goFieldName(classes map[string]*class, name string) string {
	if cl := classes[name]; cl != nil {
		return goTypeName(cl)
	}
	return name
}
func ctorName(c *class) string {
	if c.abstract || unexportedClass(c) {
		return "new" + c.name
	}
	return "New" + c.name
}
func findCtor(c *class) *method {
	for i := range c.methods {
		if c.methods[i].ctor {
			return &c.methods[i]
		}
	}
	return nil
}
func methodName(m method) string {
	if m.mods["public"] {
		return exported(m.name)
	}
	return m.name
}
func virtualMethods(c *class) []method {
	var v []method
	for _, m := range c.methods {
		if !m.ctor && (m.mods["virtual"] || m.mods["abstract"] || m.mods["override"]) {
			v = append(v, m)
		}
	}
	return v
}
func emit(u *unit, classes map[string]*class) (string, error) {
	var b strings.Builder
	// dotted drives `Name.Member` -> `Name_Member` rewriting for both enum
	// members and nested classes (Outer.Inner -> Outer_Inner).
	enums := map[string]map[string]bool{}
	for _, e := range u.enums {
		set := map[string]bool{}
		for _, m := range e.members {
			set[m] = true
		}
		enums[e.name] = set
	}
	for outer, inners := range u.nestedDotted {
		if enums[outer] == nil {
			enums[outer] = map[string]bool{}
		}
		for inner := range inners {
			enums[outer][inner] = true
		}
	}
	// The symbol table drives type-reference resolution during code generation.
	syms := buildSymbols(classes, u.interfaces, u.enums)
	b.WriteString("// Code generated by GoOOP; DO NOT EDIT.\n\n")
	b.WriteString(u.header)
	b.WriteString("\n\n")
	for _, in := range u.interfaces {
		fmt.Fprintf(&b, "type %s%s interface {\n", in.name, declaredParams(in.typeParams))
		for _, m := range in.methods {
			p, r := resolveSignature(m.params, m.returns, syms)
			fmt.Fprintf(&b, "%s(%s) %s\n", methodName(m), p, r)
		}
		b.WriteString("}\n\n")
	}
	for _, e := range u.enums {
		fmt.Fprintf(&b, "type %s int\n\nconst (\n", e.name)
		for i, m := range e.members {
			if i == 0 {
				fmt.Fprintf(&b, "%s_%s %s = iota\n", e.name, m, e.name)
			} else {
				fmt.Fprintf(&b, "%s_%s\n", e.name, m)
			}
		}
		b.WriteString(")\n\n")
		fmt.Fprintf(&b, "var %s_names = [...]string{", e.name)
		for i, m := range e.members {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q", m)
		}
		b.WriteString("}\n\n")
		fmt.Fprintf(&b, "func (e %s) String() string {\n", e.name)
		fmt.Fprintf(&b, "if int(e) >= 0 && int(e) < len(%s_names) {\nreturn %s_names[e]\n}\n", e.name, e.name)
		fmt.Fprintf(&b, "return %q\n}\n\n", e.name)
	}
	for _, c := range u.classes {
		slots := virtualMethods(c)
		gn := goTypeName(c) // Go type identifier (unexported when the class is private)
		fmt.Fprintf(&b, "type %s%s struct {\n", gn, declaredParams(c.typeParams))
		for _, ref := range allParents(c) {
			fmt.Fprintf(&b, "%s%s\n", goFieldName(classes, ref.name), ref.args)
		}
		for _, f := range c.fields {
			if f.mods["static"] {
				continue // static fields become package-level vars, not struct fields
			}
			name := f.name
			if f.mods["public"] && !f.mods["property"] {
				name = exported(name)
			}
			fmt.Fprintf(&b, "%s %s\n", name, resolveTypeExpr(f.typ, syms))
		}
		if len(slots) > 0 {
			b.WriteString("__goopSelf interface {\n")
			for _, m := range slots {
				p, r := resolveSignature(m.params, m.returns, syms)
				fmt.Fprintf(&b, "%s(%s) %s\n", methodName(m), p, r)
			}
			b.WriteString("}\n")
		}
		b.WriteString("}\n\n")
		for _, f := range c.fields {
			if !f.mods["static"] {
				continue
			}
			if f.initializer != "" {
				init, err := rewrite(f.initializer, c, classes, enums, false)
				if err != nil {
					return "", fmt.Errorf("%s.%s initializer: %w", c.name, f.name, err)
				}
				fmt.Fprintf(&b, "var %s_%s %s = %s\n\n", gn, f.name, resolveTypeExpr(f.typ, syms), init)
			} else {
				fmt.Fprintf(&b, "var %s_%s %s\n\n", gn, f.name, resolveTypeExpr(f.typ, syms))
			}
		}
		fmt.Fprintf(&b, "func (self *%s%s) __goopBind(v any) {\n", gn, usedParams(c.typeArgs))
		for _, ref := range allParents(c) {
			fmt.Fprintf(&b, "self.%s.__goopBind(v)\n", goFieldName(classes, ref.name))
		}
		if len(slots) > 0 {
			b.WriteString("self.__goopSelf = v.(interface {\n")
			for _, m := range slots {
				p, r := resolveSignature(m.params, m.returns, syms)
				fmt.Fprintf(&b, "%s(%s) %s\n", methodName(m), p, r)
			}
			b.WriteString("})\n")
		} else if c.parent == "" {
			b.WriteString("_ = v\n")
		}
		b.WriteString("}\n\n")
		for _, ctor := range constructors(c) {
			params := ctor.params
			body := ctor.body
			ctorFunc := ctorName(c)
			if len(constructors(c)) > 1 {
				ctorFunc = ctorOverloadName(c, ctor)
			}
			resolvedParams, _ := resolveSignature(params, "", syms)
			fmt.Fprintf(&b, "func %s%s(%s) *%s%s {\nself := &%s%s{}\n", ctorFunc, declaredParams(c.typeParams), resolvedParams, gn, usedParams(c.typeArgs), gn, usedParams(c.typeArgs))
			calls, rest, err := leadingSupers(body, c)
			if err != nil {
				return "", fmt.Errorf("%s constructor: %w", c.name, err)
			}
			// Build the whole constructor body, then resolve overloaded calls,
			// type references and metadata once over the complete statement list.
			var cbody strings.Builder
			for _, ref := range allParents(c) {
				call, explicit := calls[ref.name]
				if explicit {
					converted, err := rewrite(call, c, classes, enums, true)
					if err != nil {
						return "", fmt.Errorf("%s constructor: %w", c.name, err)
					}
					cbody.WriteString(converted)
					cbody.WriteString("\n")
				} else {
					parentCtor, err := zeroCtorEmitName(classes[ref.name])
					if err != nil {
						return "", fmt.Errorf("%s constructor: explicitly initialize %s because %w", c.name, ref.name, err)
					}
					fmt.Fprintf(&cbody, "self.%s = *%s%s()\n", goFieldName(classes, ref.name), parentCtor, ref.args)
				}
			}
			for _, f := range c.fields {
				if f.initializer == "" || f.mods["static"] {
					continue // static fields initialize once, at package level
				}
				converted, err := rewrite(f.initializer, c, classes, enums, false)
				if err != nil {
					return "", fmt.Errorf("%s.%s initializer: %w", c.name, f.name, err)
				}
				fieldName := f.name
				if f.mods["public"] && !f.mods["property"] {
					fieldName = exported(fieldName)
				}
				fmt.Fprintf(&cbody, "self.%s = %s\n", fieldName, converted)
			}
			restConverted, err := rewrite(rest, c, classes, enums, true)
			if err != nil {
				return "", fmt.Errorf("%s constructor: %w", c.name, err)
			}
			cbody.WriteString(restConverted)
			resolvedBody, rerr := resolveStmts(cbody.String(), newInferCtx(syms, c, ctor.params))
			if rerr != nil {
				return "", fmt.Errorf("%s constructor: %w", c.name, rerr)
			}
			b.WriteString(resolvedBody)
			b.WriteString("\nself.__goopBind(self)\nreturn self\n}\n\n")
		}
		for _, m := range c.methods {
			if m.ctor {
				continue
			}
			name := m.name
			if m.mods["public"] {
				name = exported(name)
			}
			emittedName := name
			if syms.isMangled(c.name, m.name, m.params) {
				// Part of an overload set (own or inherited): emit a distinct,
				// signature-mangled Go name. Call sites are resolved to this name
				// by the typed-IR overload pass.
				emittedName = overloadMethodName(m)
			}
			mp, mr := resolveSignature(m.params, m.returns, syms)
			if m.mods["static"] {
				fmt.Fprintf(&b, "func %s_%s%s(%s) %s {\n", gn, emittedName, declaredParams(c.typeParams), mp, mr)
			} else {
				fmt.Fprintf(&b, "func (self *%s%s) %s(%s) %s {\n", gn, usedParams(c.typeArgs), emittedName, mp, mr)
			}
			if m.hasBody {
				converted, err := rewrite(m.body, c, classes, enums, false)
				if err != nil {
					return "", fmt.Errorf("%s.%s: %w", c.name, m.name, err)
				}
				resolvedBody, rerr := resolveStmts(converted, newInferCtx(syms, c, m.params))
				if rerr != nil {
					return "", fmt.Errorf("%s.%s: %w", c.name, m.name, rerr)
				}
				b.WriteString(resolvedBody)
			} else {
				fmt.Fprintf(&b, "panic(%q)\n", "abstract method "+c.name+"."+m.name)
			}
			b.WriteString("\n}\n\n")
		}
		// Overloaded methods are resolved statically at their call sites by the
		// typed-IR overload pass; no runtime dispatcher is emitted.
		for _, f := range c.fields {
			if !f.mods["property"] {
				continue
			}
			accessor := exported(f.name)
			ftyp := resolveTypeExpr(f.typ, syms)
			fmt.Fprintf(&b, "func (self *%s%s) Get%s() %s {\nreturn self.%s\n}\n\n", gn, usedParams(c.typeArgs), accessor, ftyp, f.name)
			fmt.Fprintf(&b, "func (self *%s%s) Set%s(value %s) {\nself.%s = value\n}\n\n", gn, usedParams(c.typeArgs), accessor, ftyp, f.name)
		}
		for _, in := range c.interfaces {
			fmt.Fprintf(&b, "var _ %s = (*%s%s)(nil)\n", in, gn, usedParams(c.typeArgs))
		}
		// Metadata registry uses built-in Go values: no duplicate type
		// declarations when multiple .goop files belong to one package. A
		// private class gets an unexported metadata var so it stays package-local.
		metaName := "GoOOPMetadata" + c.name
		if unexportedClass(c) {
			metaName = "goOOPMetadata" + c.name
		}
		fmt.Fprintf(&b, "var %s = map[string]any{\n", metaName)
		fmt.Fprintf(&b, "\"class\": %q, \"abstract\": %t, \"parent\": %q,\n", c.name, c.abstract, c.parent)
		fmt.Fprintf(&b, "\"annotations\": %#v,\n", c.annotations)
		b.WriteString("\"members\": map[string][]string{\n")
		for _, m := range c.methods {
			if len(m.annotations) > 0 {
				fmt.Fprintf(&b, "%q: %#v,\n", m.name, m.annotations)
			}
		}
		for _, f := range c.fields {
			if len(f.annotations) > 0 {
				fmt.Fprintf(&b, "%q: %#v,\n", f.name, f.annotations)
			}
		}
		b.WriteString("},\n}\n")
		b.WriteString("\n")
	}
	for _, f := range u.functions {
		converted, err := rewrite(f, nil, classes, enums, false)
		if err != nil {
			return "", err
		}
		resolvedFn, rerr := resolveFunc(converted, newInferCtx(syms, nil, ""))
		if rerr != nil {
			return "", rerr
		}
		b.WriteString(resolvedFn)
		b.WriteString("\n\n")
	}
	return b.String(), nil
}
func declaredParams(params string) string {
	if params == "" {
		return ""
	}
	return "[" + params + "]"
}
func usedParams(args string) string {
	if args == "" {
		return ""
	}
	return "[" + args + "]"
}
func exported(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

type replacement struct {
	start, end int
	text       string
}

// staticSymbol maps a `Class.member` static access to its generated package-level
// symbol (`Class_member`), matching the naming used when the member is emitted.
func staticSymbol(c *class, member string) (string, bool) {
	for _, f := range c.fields {
		if f.name == member && f.mods["static"] {
			return goTypeName(c) + "_" + member, true
		}
	}
	for _, m := range c.methods {
		if m.name == member && m.mods["static"] && !m.ctor {
			name := member
			if m.mods["public"] {
				name = exported(name)
			}
			return goTypeName(c) + "_" + name, true
		}
	}
	return "", false
}

// rewrite only touches identifiers seen by the lexer, never comments or strings.
func rewrite(body string, c *class, classes map[string]*class, enums map[string]map[string]bool, constructor bool) (string, error) {
	ts := lex(body)
	var edits []replacement
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		// Enum.Member -> Enum_Member, and Outer.Inner -> Go type name of Outer_Inner.
		if members := enums[t.value]; members != nil && i+2 < len(ts) && ts[i+1].value == "." && members[ts[i+2].value] {
			repl := t.value + "_" + ts[i+2].value
			if cl := classes[repl]; cl != nil {
				repl = goTypeName(cl)
			}
			edits = append(edits, replacement{t.start, ts[i+2].end, repl})
			i += 2
			continue
		}
		// Class.member -> Class_member for static field/method access.
		if target := classes[t.value]; target != nil && i+2 < len(ts) && ts[i+1].value == "." {
			if sym, ok := staticSymbol(target, ts[i+2].value); ok {
				edits = append(edits, replacement{t.start, ts[i+2].end, sym})
				i += 2
				continue
			}
		}
		// new Outer.Inner(...) -> constructor of the flattened Outer_Inner class.
		if t.value == "new" && i+4 < len(ts) && ts[i+2].value == "." && isIdentStart(ts[i+1].value[0]) && isIdentStart(ts[i+3].value[0]) && (ts[i+4].value == "(" || ts[i+4].value == "[") {
			mangled := ts[i+1].value + "_" + ts[i+3].value
			if target := classes[mangled]; target != nil {
				if target.abstract {
					return "", fmt.Errorf("cannot instantiate abstract class %s", mangled)
				}
				edits = append(edits, replacement{t.start, ts[i+3].end, ctorName(target)})
				i += 3
				continue
			}
		}
		if t.value == "new" && i+2 < len(ts) && isIdentStart(ts[i+1].value[0]) && (ts[i+2].value == "(" || ts[i+2].value == "[") {
			name := ts[i+1].value
			base := "New" + name
			if target := classes[name]; target != nil {
				if target.abstract {
					return "", fmt.Errorf("cannot instantiate abstract class %s", name)
				}
				base = ctorName(target)
			}
			edits = append(edits, replacement{t.start, ts[i+1].end, base})
			i++
			continue
		}
		if t.value == "super" {
			if c != nil && i+3 < len(ts) && ts[i+1].value == "." && ts[i+3].value == "(" && containsParent(c, ts[i+2].value) {
				if !constructor {
					return "", fmt.Errorf("super.%s(...) can only initialize a parent inside a constructor", ts[i+2].value)
				}
				chosenParent := ts[i+2].value
				actual := classes[chosenParent]
				if actual == nil {
					return "", fmt.Errorf("unknown superclass %s", chosenParent)
				}
				chosen := ctorName(actual) // overload suffix resolved by the typed-IR pass
				refArgs := ""
				for _, ref := range allParents(c) {
					if ref.name == chosenParent {
						refArgs = ref.args
					}
				}
				edits = append(edits, replacement{t.start, ts[i+2].end, "self." + goFieldName(classes, chosenParent) + " = *" + chosen + refArgs})
				i += 2
				continue
			}
			if c == nil || c.parent == "" {
				return "", fmt.Errorf("super used without a parent class")
			}
			if i+1 < len(ts) && ts[i+1].value == "(" {
				if !constructor {
					return "", fmt.Errorf("super(...) can only be called in a constructor")
				}
				parent := classes[c.parent]
				if parent == nil {
					return "", fmt.Errorf("unknown superclass %s", c.parent)
				}
				chosen := ctorName(parent) // overload suffix resolved by the typed-IR pass
				edits = append(edits, replacement{t.start, t.end, "self." + goFieldName(classes, c.parent) + " = *" + chosen + c.parentArgs})
				continue
			}
			if i+2 < len(ts) && ts[i+1].value == "." && containsParent(c, ts[i+2].value) {
				if i+4 < len(ts) && ts[i+3].value == "." {
					parent := classes[ts[i+2].value]
					if parent != nil {
						for _, m := range parent.methods {
							if m.name == ts[i+4].value && m.mods["private"] {
								return "", fmt.Errorf("%s: cannot access private superclass method %s.%s", c.name, parent.name, m.name)
							}
						}
						for _, f := range parent.fields {
							if f.name == ts[i+4].value && f.mods["private"] {
								return "", fmt.Errorf("%s: cannot access private superclass field %s.%s", c.name, parent.name, f.name)
							}
						}
					}
				}
				edits = append(edits, replacement{t.start, ts[i+2].end, "self." + goFieldName(classes, ts[i+2].value)})
				i += 2
				continue
			}
			edits = append(edits, replacement{t.start, t.end, "self." + goFieldName(classes, c.parent)})
			continue
		}
		if t.value == "this" {
			if c == nil {
				return "", fmt.Errorf("this used outside a class")
			}
			if i+2 < len(ts) && ts[i+1].value == "." {
				name := ts[i+2].value
				target := "self." + name
				own := false
				for _, f := range c.fields {
					if f.name == name {
						own = true
					}
				}
				for _, m := range c.methods {
					if m.name == name {
						own = true
					}
				}
				if !own {
					checked := map[string]bool{}
					var inspect func(*class) error
					inspect = func(parent *class) error {
						if parent == nil || checked[parent.name] {
							return nil
						}
						checked[parent.name] = true
						for _, f := range parent.fields {
							if f.name == name {
								if f.mods["private"] {
									return fmt.Errorf("%s: cannot access private superclass field %s.%s", c.name, parent.name, name)
								}
								if f.mods["public"] {
									target = "self." + exported(name)
								}
							}
						}
						for _, m := range parent.methods {
							if m.name == name {
								if m.mods["private"] {
									return fmt.Errorf("%s: cannot access private superclass method %s.%s", c.name, parent.name, name)
								}
								if m.mods["public"] {
									target = "self." + exported(name)
								}
							}
						}
						for _, ref := range allParents(parent) {
							if err := inspect(classes[ref.name]); err != nil {
								return err
							}
						}
						return nil
					}
					for _, ref := range allParents(c) {
						if err := inspect(classes[ref.name]); err != nil {
							return "", err
						}
					}
				}
				for _, f := range c.fields {
					if f.name == name && f.mods["public"] {
						target = "self." + exported(name)
					}
				}
				for _, m := range c.methods {
					if m.name == name && !m.ctor && m.mods["public"] {
						target = "self." + exported(name)
					}
				}
				for _, m := range virtualMethods(c) {
					if m.name == name {
						target = "self.__goopSelf." + methodName(m)
					}
				}
				edits = append(edits, replacement{t.start, ts[i+2].end, target})
				i += 2
				continue
			}
			edits = append(edits, replacement{t.start, t.end, "self"})
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var b strings.Builder
	offset := 0
	for _, edit := range edits {
		if edit.start < offset {
			continue
		}
		b.WriteString(body[offset:edit.start])
		b.WriteString(edit.text)
		offset = edit.end
	}
	b.WriteString(body[offset:])
	return b.String(), nil
}
