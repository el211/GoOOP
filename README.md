# GoOOP

**Classical object-oriented syntax for Go — compiled to native Go.**

GoOOP is a small, experimental source-to-source compiler. Write `.goop` files with classes, constructors, `extends`, `abstract`, generics, constructor/method overloads, metadata annotations, `virtual`, `override`, `this`, `super` and `new`, then run `goop run`, `goop build`, `goop test`, or `goop check`. Generated Go source lives **only in an automatically deleted operating-system temp directory**. GoOOP supplies it to the standard Go toolchain via `-overlay`; it does **not** create generated `.go` files in your project unless you explicitly request `goop generate`. No JVM and no fork of Go are needed.

> **Status:** v0.3.0 experimental compiler. Go is already object-oriented through structs, methods, interfaces and composition. GoOOP adds an *optional classical-OOP syntax layer*, not new native language semantics.

## Why GoOOP?

Go is deliberately minimal: it gives you structs, methods, interfaces and
composition, but no `class`, no constructors, no `extends`, no `super`, and no
annotations. That minimalism is a strength for Go, but developers coming from
Java, C#, Kotlin or TypeScript often want the familiar classical-OOP vocabulary
for modelling domains — especially inheritance hierarchies, constructors and
metadata.

GoOOP exists to bridge that gap **without leaving the Go ecosystem**:

- **Familiar syntax, zero runtime cost.** You write classical OOP; it compiles
  down to ordinary, idiomatic Go (struct embedding, methods, interfaces). There
  is no VM, no reflection-heavy framework, and no GoOOP runtime to ship.
- **No fork of Go, no new toolchain to trust.** GoOOP is a thin source-to-source
  pass that hands generated Go to the *standard* `go` toolchain. Everything the
  Go community already knows — modules, `go test`, the race detector, `go vet`,
  profiling — keeps working.
- **Your repository stays clean.** Generated Go lives only in a temporary build
  overlay that is deleted afterwards, so you commit `.goop`, not machine output.
- **Incremental adoption.** `.goop` and plain `.go` files coexist in the same
  package. You can use classes where they help and drop to raw Go everywhere
  else.

In short: keep Go's performance, tooling and deployment story, but express
object models in the classical style when that is the clearer way to say it.

## How GoOOP works

GoOOP is a **transpiler** (source-to-source compiler), not an interpreter or a
language runtime. The pipeline for `goop run` / `build` / `test` / `check` is:

1. **Discover.** From the current directory, walk upward to the enclosing
   `go.mod` and collect every `.goop` file in each package (skipping nested
   modules).
2. **Lex & parse.** Each `.goop` file is tokenized and parsed while preserving
   byte offsets, so positions in the generated Go map back to your source.
3. **Resolve the class model.** Classes, interfaces, inheritance (`extends` /
   `implements`), generics, overloads and annotations are validated together
   across the whole package (cyclic inheritance, abstract-method coverage,
   override signatures, etc.).
4. **Rewrite to Go.** The OOP constructs are lowered to native Go: classes →
   structs with embedding, constructors → `NewT(...)` functions, `this`/`super`
   → receiver/embedded-field access, `virtual`/`override` → self-dispatch,
   annotations → `GoOOPMetadata*` maps. The result is `gofmt`-formatted Go.
5. **Build via overlay.** The generated Go is written to an OS temp directory and
   handed to the standard Go toolchain through `go -overlay=<json>`, so `go`
   compiles it as virtual `*_goop.go` files *alongside* your real `.go` files.
6. **Clean up.** On success **and** failure the temp directory is removed. Your
   project tree is never touched — unless you explicitly run `goop generate`,
   which writes the generated Go next to your `.goop` for inspection or native
   interop.

Because the heavy lifting is done by the real Go compiler, GoOOP adds syntax,
not semantics: anything Go can't express, GoOOP can't magically add (see
[Boundaries of this release](#boundaries-of-this-release)).

## Install

Requires Go 1.22 or newer:

```sh
git clone https://github.com/el211/GoOOP.git
cd GoOOP
go install ./cmd/goop
# or run without installation: go run ./cmd/goop help
```

## Quick start

```sh
goop init example.com/animals ./myapp
cd myapp
goop run
# Rex: Woof!
```

Or see [`examples/animals/animals.goop`](examples/animals/animals.goop):

```go
package main

import "fmt"

interface Speaker {
    Speak() string;
}

abstract class Animal {
    private name string

    constructor(name string) {
        this.name = name
    }

    abstract Speak() string;

    Describe() string {
        return this.name + " says " + this.Speak()
    }
}

class Dog extends Animal implements Speaker {
    constructor(name string) { super(name) }
    override Speak() string { return "woof" }
}

func main() {
    dog := new Dog("Rex")
    fmt.Println(dog.Describe()) // Rex says woof
}
```

```sh
# From the repository root:
go build -o ./goop ./cmd/goop
cd examples/animals
../../goop run
# Rex says woof
# Milo says meow
```

By default, the project directory remains clean:

```text
myapp/
├── go.mod
├── app.goop
└── services/
    └── account.goop
```

`goop run`, `build`, `test`, and `check` generate formatted Go into an OS temp directory and supply a JSON build overlay to `go`. The Go toolchain treats the generated Go as virtual `*_goop.go` files alongside your regular `.go` files. On success **and failure**, GoOOP removes the temporary directory. `goop build` produces a conventional native executable in the current source directory when the current package is `main`; it creates no generated source there. Working from a subdirectory is supported: the compiler searches upward for the enclosing `go.mod` and processes its GoOOP packages (excluding nested modules).

`goop generate` is **opt-in source export** for debugging or native Go interoperability: only this command writes `animals_goop.go` beside `animals.goop`. `goop clean` removes these exported files only if they retain the `// Code generated by GoOOP; DO NOT EDIT.` header and have a corresponding `.goop` file; it never removes arbitrary similarly named user files. There is no persistent GoOOP compiler cache to clean.

The Go tool's implicit `go test` vet step cannot read virtual source paths reliably, so `goop test` invokes `go test -vet=off -overlay=... ./...`. The sources are still compiled and tests run. If you need an explicit `go vet` pass over generated code, run `goop generate && go vet ./... && goop clean`. This is a limitation of the overlay integration, not a claim of full language parity. GoOOP requires Go 1.22 or later.

## Commands

| Command | Action |
| --- | --- |
| `goop init <module> [directory]` | Create a Go module and runnable starter `.goop` file. |
| `goop run [args...]` | Compile using temporary overlay and run with program arguments. |
| `goop build` | Build using temporary overlay; emit an executable if the current package is `main`. |
| `goop test [flags...]` | Run tests using temporary overlay (`-vet=off`; see above). |
| `goop check` | Build-check using temporary overlay. |
| `goop generate [paths...]` | **Explicitly** export generated Go alongside `.goop` inputs. |
| `goop clean` | Remove GoOOP-authored exported Go files; temporary output is always removed automatically. |
| `goop version` | Print version. |

## Editor support (IntelliJ / JetBrains IDEs)

An IntelliJ Platform plugin in [`idea-plugin/`](idea-plugin/) teaches IntelliJ
IDEA and other JetBrains IDEs to recognize `.goop` files:

- `.goop` registered as a first-class language with its own file type and icon
- Syntax highlighting for the OOP keywords (`class`, `interface`, `abstract`,
  `extends`, `implements`, `constructor`, `override`, `new`, `this`, `super`, …),
  Go keywords, built-in types, strings, numbers, comments and operators
- Class-name highlighting after `class` / `interface` / `extends` / `implements` / `new`
- Brace / paren / bracket matching and `//` + `/* */` comment toggling
- A **Settings → Editor → Color Scheme → GoOOP** page to recolor every token

It is a standard [IntelliJ Platform Gradle Plugin](https://plugins.jetbrains.com/docs/intellij/tools-intellij-platform-gradle-plugin.html)
project (Kotlin, hand-written lexer + flat PSI parser — no generated sources).

### Build the plugin

Requires a JDK 21 to **run** Gradle (Gradle 8.10 does not run on Java 24+).
The easiest path is to open `idea-plugin/` in IntelliJ and use the Gradle tool
window; from a terminal:

```sh
cd idea-plugin
JAVA_HOME=/path/to/jdk-21 gradle buildPlugin   # or ./gradlew once a wrapper exists
# → build/distributions/goop-intellij-0.1.0.zip
```

Use the `runIde` task instead to launch a sandbox IDE with the plugin already
loaded (no install needed) for quick testing.

### Install the plugin

In any JetBrains IDE: **Settings → Plugins → ⚙ → Install Plugin from Disk…**,
pick `idea-plugin/build/distributions/goop-intellij-0.1.0.zip`, and restart.

See [`idea-plugin/README.md`](idea-plugin/README.md) for details.

### Known issues & fixes

#### `@` annotations were underlined in red (fixed)

- **Symptom:** In the IntelliJ plugin, an annotation such as `@Serializable`
  showed a red error squiggle under the `@`, even though the annotation is valid
  GoOOP and compiles fine.
- **Cause:** This was purely a plugin highlighting bug, **not** a compiler error.
  The plugin's lexer had no rule for the `@` character, so it fell through to the
  "bad character" branch and was flagged as an error token.
- **Fix:** The lexer now recognizes `@` followed by an identifier as a dedicated
  **annotation token** (covering both `@Name` and `@Name(...)`), highlighted as
  metadata — the same style IntelliJ uses for Java annotations like `@Override`.
- **To get the fix:** rebuild the plugin (`gradle buildPlugin`) and reinstall the
  zip via **Settings → Plugins → ⚙ → Install Plugin from Disk…**, then restart.
  If IntelliJ keeps the old copy because the version string is unchanged,
  uninstall the previous GoOOP plugin first.

> Note: this only affected editor highlighting. `goop run`/`build`/`test` have
> always accepted `@Name` and `@Name(...)` on class, interface, field and method
> declarations (see [Annotations / metadata](#language-support-v030)).

## Language support (v0.3.0)

GoOOP aims for **Java-inspired syntax over native Go**, not source compatibility
with Java or a JVM. It accepts ordinary Go expressions and statements inside
method bodies, plus the explicit GoOOP constructs documented below.

| Feature | Status and exact semantics |
| --- | --- |
| Classes / single inheritance | Supported: Go struct embedding and constructors. |
| Multiple inheritance | Supported for distinct superclass embeddings: `class C extends A, B`. Shared method names must be explicitly overridden; each parent is an independently embedded Go value, not a Java object identity. |
| Abstract classes / interfaces | Supported with compilation checks and Go interfaces. |
| Class generics | Supported using Go type parameters, e.g. `class Box[T any]` and `new Box[int](3)`; superclass type arguments such as `extends Base[string]` supported. |
| Constructor overloads | Supported **by argument count and type**, resolved statically by the typed IR. `new T(...)` and `super(...)` are matched against the constructor set using inferred argument types; each overload emits a distinct signature-mangled symbol. |
| Method overloads | Supported **by argument count and type** (Java-style, resolved at compile time). Overloads may differ in parameter types and return type; they must be concrete, nonvirtual instance methods. Each call site is resolved against the symbol table and typed AST and rewritten to a distinct generated method — there is no runtime dispatcher. Ambiguous, unmatched or uninferrable calls are rejected with a GoOOP diagnostic. |
| Override signatures | Parameters must match by **type** (names ignored); the return type must be identical or a **covariant subtype** (checked through the class hierarchy). Generic superclass type arguments are substituted during validation. Covariant returns are accepted for non-virtual overrides; a covariant return on a *virtual/abstract* method is rejected with a diagnostic (the dispatch bridge is a planned milestone). |
| Inherited-member resolution | A subclass sees the **merged** overload set of its ancestors: inherited `Show(int)` and an own `Show(string)` coexist instead of the child hiding the parent (as raw Go embedding would). Resolution traverses the full hierarchy, substitutes generics, collapses overrides, omits private members, and mangles every member of an overload family to a collision-free Go name so promotion works. |
| Field initializers | Supported: fields initialize in declaration order, after superclass construction and before the class constructor body. |
| `static` members | Supported: `static` fields become package-level `var ClassName_field` initialized once; `static` methods become `func ClassName_Method(...)`. Access both as `ClassName.member` inside `.goop`. Static members cannot combine with `abstract`/`virtual`/`override` or be overloaded. |
| Enums | Supported: `enum Color { Red, Green, Blue }` lowers to a `type Color int` with `iota` constants, a generated `String()` method, and `Color.Member` access. Enum members with associated data or methods are not yet supported. |
| Properties | Supported: `property name string` generates `GetName()`/`SetName(value string)` over an unexported backing field. Supports an initializer (`property name string = "anon"`) and `this.name` access inside the class. Cannot be `static`. |
| `final` / sealed | Supported: `final class X` cannot be extended; a `final` method cannot be overridden; both are rejected at compile time. `final` cannot combine with `abstract`/`static` or constructors. |
| Nested classes | Supported: a `class Inner { ... }` declared inside `Outer` is flattened to a top-level `Outer_Inner` type. Reference it as `Outer.Inner` (e.g. `new Outer.Inner(...)`), which lowers to `Outer_Inner`. These are *static* nested classes — no implicit reference to an outer instance. |
| Annotations / metadata | `@Name` and `@Name(...)` accepted on class, interface, field and method declarations; class/annotated-member metadata is available through `GoOOPMetadataClassName`. Annotations are metadata, **not executable decorators**. |
| Visibility (members) | `private`, `protected`, `public` parsed; inherited private access via `this` and direct named `super` access receives diagnostics. Public field/method symbols use Go capitalization. Full Java access enforcement is not guaranteed across arbitrary Go expressions or native `.go` sources. |
| Visibility (classes) | Follows Java's class rules, enforced by the typed-IR frontend. **Top-level** classes may only be `public` or package-private (default); `private`/`protected` top-level classes are rejected. **Nested** classes accept all four. `public`/`protected` classes are emitted as **exported** Go types; **package-private (default) and `private` classes are emitted as unexported** Go types — their type name, `NewX` constructor, `GoOOPMetadataX` and every resolved reference (field types, parameters, return types, generic arguments, local-variable types) are rewritten consistently to the unexported form, so native Go in other packages cannot reach them. A `private` nested class is additionally restricted to its enclosing top-level class (compiler-enforced) and may be used as a superclass there. To expose a class (and its `NewX`/metadata) to native Go in another package, mark it `public`. |
| Cross-file inheritance | All `.goop` declarations **within a Go package directory** are resolved together. Imported cross-package classes use normal Go package APIs, not implicit `extends` across packages. |
| Polymorphism | Existing `virtual` / `override` generated self-dispatch; **constructor-time dispatch does not emulate Java's partially initialized subclass semantics**. |
| Native integration | Standard Go module, imports, structs and libraries; ephemeral `go -overlay` in `goop build`, `run`, `test`, `check`. |

### Multiple inheritance example

```go
class Named {
    private name string
    constructor(name string) { this.name = name }
    Name() string { return this.name }
}
class Numbered {
    private number int = 7
    constructor(n int) { this.number = n }
    Number() int { return this.number }
}
class Combined extends Named, Numbered {
    constructor(name string, n int) {
        super(name)          // first parent, Named
        super.Numbered(n)    // explicitly initialize second parent
    }
    Describe() string { return this.Name() }
}
```

A superclass not explicitly initialized must have a zero-argument constructor.
`super(...)` / `super.Parent(...)` calls must appear before other constructor
statements. When two parents promote the same method, add an explicit override
to remove Go's selector ambiguity. The compiler rejects cyclic inheritance.

### Overloads and generic declarations

```go
@Serializable
class Box[T any] {
    private value T
    constructor(value T) { this.value = value }
    Get() T { return this.value }
}
class Calculator {
    Add(x int) int { return x + 1 }          // arity overload
    Add(x int, y int) int { return x + y }   // arity overload
}
func example() {
    box := new Box[string]("hello")
    _ = box.Get()
    _ = new Calculator().Add(2, 3)
}
```

The metadata for `Box` is exposed as `GoOOPMetadataBox`, a Go `map[string]any`.
`new` is translated only in `.goop` sources.

#### Same-arity overloading (resolved by type)

Overloads may share the same arity and differ only by parameter type; the typed
IR infers each argument's type and selects the most specific match at compile
time (there is no runtime dispatch):

```go
public class Printer {
    constructor() {}
    public Show(v int) string { return fmt.Sprintf("int:%d", v) }
    public Show(v string) string { return "str:" + v }
    public Show(v bool) string { return fmt.Sprintf("bool:%t", v) }
}

public class Point {
    private label string
    constructor(x int) { this.label = fmt.Sprintf("n%d", x) }  // ctor overload
    constructor(s string) { this.label = "s" + s }             // same arity, by type
    Label() string { return this.label }
}

func main() {
    p := new Printer()
    fmt.Println(p.Show(42))    // int:42
    fmt.Println(p.Show("hi"))  // str:hi
    fmt.Println(p.Show(true))  // bool:true
    fmt.Println(new Point(7).Label())    // n7
    fmt.Println(new Point("x").Label())  // sx
}
```

A call that matches no overload, matches several equally well, or whose argument
type cannot be inferred is rejected during GoOOP semantic analysis (e.g.
`ambiguous overloaded call to Show ...`), not by a later Go compiler error.

The complete runnable package-wide example is in
[`examples/advanced/`](examples/advanced/). From the repository root:

```sh
go build -o ./goop ./cmd/goop
cd examples/advanced
../../goop run
# Animal: woof2
# 3
# 9
# [Serializable]
```

### Boundaries of this release

**Not yet implemented:** same-arity type-based overload resolution, overloaded
virtual/interface methods, executable decorators or annotation processors,
Java-style operator overloading, arbitrary Java expression syntax, full
Java-class private/protected enforcement, cross-**package** inheritance,
method-specific type parameters beyond normal native Go restrictions,
constructor-time Java virtual dispatch, and full `java.lang.reflect` parity.
These require a typed expression frontend and separate semantics; they cannot
be provided reliably by a text-rewrite pass alone. Ordinary `.go` and `.goop`
files can coexist, but Go source can access package-level declarations according
to Go's native rules.

### Roadmap to full OOP parity

A feature-by-feature view of what classical OOP (Java / C# style) offers versus
GoOOP today, grouped by how hard each is given that GoOOP rewrites to Go.

**A. Rewrite-friendly — planned incremental work**

| Feature | Status | Notes |
| --- | --- | --- |
| `static` fields / methods | ✅ supported | Lowered to package-level vars/funcs; access via `Class.member`. |
| Enums (named constants) | ✅ supported | Lowered to a typed `int` + `iota` constants, `String()`, and `Enum.Member` access. Methods/fields on enums not yet supported. |
| Properties (getters/setters sugar) | ✅ supported | `property x T` generates `GetX`/`SetX` over an unexported backing field. |
| `final` / sealed enforcement | ✅ supported | `final class`/`final` method rejected at compile time when extended/overridden. |
| Nested / inner classes | ✅ supported | Flattened to top-level `Outer_Inner` types; reference via `Outer.Inner`. Static-nested semantics (no implicit outer instance). |

**B. Needs a typed expression frontend (semantic IR, not text rewrite)**

| Feature | Status | Notes |
| --- | --- | --- |
| Same-arity overloading by type | ✅ supported | Resolved at compile time by the typed IR (argument type inference + specificity). Within a package/inheritance chain; cross-**package** overload resolution and overloaded virtual/interface methods remain future work. |
| Class-level visibility (public/private/protected) | ✅ supported | Java top-level rules enforced; private nested classes access-checked and emitted unexported. |
| Full member `private` / `protected` enforcement | ⚠️ partial | Diagnostics exist; full enforcement across arbitrary Go expressions needs a resolver. |
| Constructor-time virtual dispatch | ❌ | Java's partially-initialized subclass semantics. |
| Covariant return types | ✅ supported (non-virtual) | Validated through the class hierarchy (pointer covariance `*Derived <: *Base`, interface implementation). Works via Go shadowing for non-virtual overrides; virtual/abstract covariant overrides are rejected pending the dispatch-bridge milestone. |
| Inherited overload merging | ✅ supported | Merged across single, multi-level and multiple/diamond inheritance; same-signature promotion from two parents is reported as ambiguous and requires an explicit override. |
| Overloaded virtual / interface methods | ❌ | Requires typed dispatch tables. |

**C. Hard or un-idiomatic in Go — may never reach 1:1 parity**

| Feature | Status | Notes |
| --- | --- | --- |
| Operator overloading | ❌ | Go has none; would need expression lowering. |
| Exceptions (`try`/`catch`/`finally`) | ❌ | Go idiom is error values + `panic`/`recover`. |
| Executable annotations / decorators | ❌ | Annotations are metadata only. |
| `java.lang.reflect` parity | ❌ | Go reflection differs fundamentally. |
| Cross-**package** inheritance | ❌ | Resolution is per Go package. |

**Design stance:** GoOOP intends to be a *pragmatic* classical-OOP layer over
idiomatic Go, not a Java clone. Category A is a normal backlog; category B is
gated on the typed IR below; category C fights Go's design and is out of scope
unless it can lower cleanly.

## Compiler architecture (typed IR)

GoOOP is moving from string rewriting to a resolution-driven pipeline:

```
.goop source → lexer → parser (AST) → symbol table → type/identifier
resolution → semantic analysis → Go code generation → gofmt
```

- **Symbol table** (`internal/compiler/symbols.go`): one table per package holding
  every class, interface and enum with its source location, visibility and the
  Go identifier it is emitted as (`GoName`). Package-private and `private`
  classes get unexported Go names; `public`/`protected` stay exported.
- **Type & identifier resolution** (`internal/compiler/resolve.go`): type
  references are parsed with `go/parser` and rewritten on the **AST** — never by
  regular-expression name replacement. The resolver handles pointers, slices,
  maps, channels, generic instantiations and qualified nested classes
  (`Outer.Inner`), and is applied to field types, parameters, return types,
  constructor signatures, generic arguments and in-body type positions
  (`var x T`, composite literals, type assertions) plus the generated
  `GoOOPMetadataX` references.
- **Consistency guarantee:** a class and all of its generated symbols — the type,
  `NewX`/`newX`, `GoOOPMetadataX`, receivers, embeddings and every resolved
  reference — share one visibility, so a package-private implementation never
  leaks through an exported symbol.
- **Overload resolution** (`internal/compiler/overload.go`): overloaded methods
  and constructors are matched at compile time. Argument expressions are type-
  inferred over a scope (parameters, locals, fields, `this`, `new`, method
  returns); each candidate is scored by assignability and specificity; the
  winner's distinct signature-mangled name is written at the call site. No
  runtime `...any` dispatcher is emitted.
- **Inherited-member resolution** (`SymbolTable.mergedMethods`): resolves the
  overload set visible on a class across its whole hierarchy — substituting
  generic superclass type arguments, collapsing overrides, and omitting private
  members. A package-wide pass then marks every member of an overload family so
  parent and child emit and call consistent mangled names, and covariant
  overrides are validated through the hierarchy (`SymbolTable.isSubtype`).

Delivered incrementally; milestones so far resolve package-private classes
end-to-end, perform static same-arity overload resolution, and unify inherited
members with covariant-return validation (see `resolve_test.go`,
`overload_test.go`, `inherit_test.go`, and integration tests such as
`TestPackagePrivateClassFullyUnexportedViaTypedIR`,
`TestSameArityMethodOverloadByType` and `TestMultipleInheritanceMergedOverloads`).
The remaining string-based lowering of `new`/`this`/`super`/static access — and
the virtual-dispatch bridge needed for covariant returns on virtual methods — are
being migrated onto the same resolver in subsequent milestones.

## Roadmap

1. ✅ Typed IR foundation: symbol table + AST-based type/identifier resolution.
2. ✅ True package-private unexported emission through resolved references.
3. ✅ Same-arity method/constructor overloading via static type resolution.
4. ✅ Unified inherited-member resolution, merged overload sets, and covariant
   return validation (virtual-dispatch bridge for covariant returns pending).
5. Full source-level visibility checks and constructor dispatch semantics.
6. Operator expression lowering and opt-in executable annotation processors.
7. Language-server diagnostics, source maps and incremental package graph.

## Development

```sh
go test ./...
go build -o ./goop ./cmd/goop
(cd examples/animals && ../../goop run && ../../goop test && ../../goop check && ../../goop build)
```

Architecture: `cmd/goop` provides the CLI, while `internal/compiler` handles byte-offset-preserving lexing, `.goop` parsing, class validation, a package **symbol table** and **AST-based type resolution** (see [Compiler architecture](#compiler-architecture-typed-ir)), and formatted Go emission. Contributions and small reproducible examples are welcome.

**Implementation note:** GoOOP uses the standard Go build overlay (not a copied project tree), preserving relative module references, mixed native Go packages and build assets. External tools that do not accept `go -overlay` will not see virtual generated Go until you explicitly run `goop generate`.
