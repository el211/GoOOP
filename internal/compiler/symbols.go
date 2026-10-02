package compiler

// Symbol is a resolved top-level declaration within one GoOOP package.
type Symbol struct {
	Name       string // GoOOP source name (map key)
	GoName     string // emitted Go identifier (visibility-aware)
	Kind       string // "class", "interface" or "enum"
	Visibility string // "", "public", "protected" or "private"
	Class      *class // non-nil when Kind == "class"
	Pos        int    // source byte offset of the declaration, for diagnostics
}

// SymbolTable resolves GoOOP declaration names within a single package. It is
// the backbone of type and identifier resolution: the code generator consults
// it instead of rewriting identifiers textually.
type SymbolTable struct {
	byName map[string]*Symbol
}

func newSymbolTable() *SymbolTable {
	return &SymbolTable{byName: map[string]*Symbol{}}
}

func (s *SymbolTable) add(sym *Symbol) { s.byName[sym.Name] = sym }

// Lookup returns the symbol declared under a GoOOP name in this package.
func (s *SymbolTable) Lookup(name string) (*Symbol, bool) {
	sym, ok := s.byName[name]
	return sym, ok
}

// classFor returns the class declared under a GoOOP name, or nil.
func (s *SymbolTable) classFor(name string) *class {
	if sym, ok := s.byName[name]; ok {
		return sym.Class
	}
	return nil
}

// buildSymbols constructs the package symbol table from the resolved class map
// plus the interface and enum declarations. A class's GoName encodes its
// visibility, so every consumer that resolves a type reference through the table
// gets a Go identifier consistent with the class's access level.
func buildSymbols(classes map[string]*class, interfaces []iface, enums []enumDecl) *SymbolTable {
	st := newSymbolTable()
	for name, c := range classes {
		st.add(&Symbol{
			Name:       name,
			GoName:     goTypeName(c),
			Kind:       "class",
			Visibility: c.visibility,
			Class:      c,
			Pos:        c.pos,
		})
	}
	for _, in := range interfaces {
		st.add(&Symbol{Name: in.name, GoName: in.name, Kind: "interface"})
	}
	for _, e := range enums {
		st.add(&Symbol{Name: e.name, GoName: e.name, Kind: "enum"})
	}
	return st
}
