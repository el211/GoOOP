package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstructorOverloadsByArity(t *testing.T) {
	src := `package main
import "fmt"
class Dog {
    private name string = "Milo"
    private age int = 1
    constructor() { }
    constructor(name string) { this.name = name }
    constructor(name string, age int) { this.name=name; this.age=age }
    Describe() string {return fmt.Sprintf("%s:%d",this.name,this.age)}
}
func main(){
    fmt.Println(new Dog().Describe())
    fmt.Println(new Dog("Rex").Describe())
    fmt.Println(new Dog("Rex",4).Describe())
}`
	generated, err := Compile("overloads.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/overloads\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "overloads_goop.go"), generated, 0644)
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", src, err, out)
	}
	if string(out) != "Milo:1\nRex:1\nRex:4\n" {
		t.Fatalf("unexpected output %q", out)
	}
}

func runGoop(t *testing.T, src, mod string) string {
	t.Helper()
	out, err := Compile("ov.goop", []byte(src))
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/"+mod+"\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ov_goop.go"), out, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code failed: %v\n%s\n--- generated ---\n%s", err, result, out)
	}
	return string(result)
}

func TestSameArityMethodOverloadByType(t *testing.T) {
	src := `package main
import "fmt"
public class Printer {
    constructor() {}
    public Show(v int) string { return fmt.Sprintf("int:%d", v) }
    public Show(v string) string { return "str:" + v }
    public Show(v bool) string { return fmt.Sprintf("bool:%t", v) }
}
func main() {
    p := new Printer()
    fmt.Println(p.Show(42))
    fmt.Println(p.Show("hi"))
    fmt.Println(p.Show(true))
}`
	if got := runGoop(t, src, "samemeth"); got != "int:42\nstr:hi\nbool:true\n" {
		t.Fatalf("got %q", got)
	}
}

func TestSameArityConstructorOverloadByType(t *testing.T) {
	src := `package main
import "fmt"
public class Point {
    private label string
    constructor(x int) { this.label = fmt.Sprintf("n%d", x) }
    constructor(s string) { this.label = "s" + s }
    Label() string { return this.label }
}
func main() {
    fmt.Println(new Point(7).Label())
    fmt.Println(new Point("x").Label())
}`
	if got := runGoop(t, src, "samector"); got != "n7\nsx\n" {
		t.Fatalf("got %q", got)
	}
}

func TestOverloadByUserTypeAndNil(t *testing.T) {
	src := `package main
import "fmt"
public class Box {
    constructor() {}
    Hold(d *Box) string { return "box" }
    Hold(n int) string { return fmt.Sprintf("n%d", n) }
}
func main() {
    b := new Box()
    fmt.Println(b.Hold(b))
    fmt.Println(b.Hold(5))
}`
	if got := runGoop(t, src, "usertype"); got != "box\nn5\n" {
		t.Fatalf("got %q", got)
	}
}

func TestOverloadDifferentReturnTypes(t *testing.T) {
	src := `package main
import "fmt"
public class Conv {
    constructor() {}
    To(v int) int { return v + 1 }
    To(v string) string { return v + "!" }
}
func main() {
    c := new Conv()
    fmt.Println(c.To(1))
    fmt.Println(c.To("a"))
}`
	if got := runGoop(t, src, "rettypes"); got != "2\na!\n" {
		t.Fatalf("got %q", got)
	}
}

func TestAmbiguousOverloadDiagnostic(t *testing.T) {
	src := `package main
public class A { constructor() {} }
public class B { constructor() {} }
public class C {
    constructor() {}
    Take(a *A) int { return 1 }
    Take(b *B) int { return 2 }
    Use() int { return this.Take(nil) }
}
func main() { _ = new C().Use() }`
	_, err := Compile("ambig.goop", []byte(src))
	if err == nil || !contains(err.Error(), "ambiguous overloaded call to Take") {
		t.Fatalf("expected ambiguity diagnostic, got: %v", err)
	}
}

func TestNoMatchingOverloadDiagnostic(t *testing.T) {
	src := `package main
public class P {
    constructor() {}
    Go(v int) int { return v }
    Go(v string) int { return 0 }
    Use() int { return this.Go(true) }
}
func main() { _ = new P().Use() }`
	_, err := Compile("nomatch.goop", []byte(src))
	if err == nil || !contains(err.Error(), "no matching overload of Go") {
		t.Fatalf("expected no-match diagnostic, got: %v", err)
	}
}

func TestDuplicateOverloadSignatureRejected(t *testing.T) {
	src := `package main
public class D {
    constructor() {}
    Same(v int) int { return v }
    Same(v int) int { return v + 1 }
}
func main() { _ = new D() }`
	_, err := Compile("dup.goop", []byte(src))
	if err == nil || !contains(err.Error(), "duplicate overload signature") {
		t.Fatalf("expected duplicate-signature diagnostic, got: %v", err)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestOverloadResolvedAcrossFiles(t *testing.T) {
	// The overloaded class is declared in one file and called from another in
	// the same package; resolution uses the package-wide symbol table.
	sources := map[string][]byte{
		"calc.goop": []byte(`package main
public class Calc {
    constructor() {}
    Op(a int) int { return a + 1 }
    Op(a int, b int) int { return a * b }
}`),
		"main.goop": []byte(`package main
import "fmt"
func main() {
    c := new Calc()
    fmt.Println(c.Op(10))
    fmt.Println(c.Op(3, 4))
}`),
	}
	outputs, err := CompileFiles(sources)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/xfile\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for path, out := range outputs {
		if err := os.WriteFile(filepath.Join(dir, strings.TrimSuffix(path, ".goop")+"_goop.go"), out, 0644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code failed: %v\n%s", err, result)
	}
	if string(result) != "11\n12\n" {
		t.Fatalf("got %q", result)
	}
}

func TestMethodOverloadsDistinctArity(t *testing.T) {
	src := `package main
import "fmt"
class Calculator {
    public Add(a int) int {return a+1}
    public Add(a int,b int) int {return a+b}
    constructor() {}
    Summary() int {return this.Add(2)+this.Add(3,4)}
}
func main(){
    calc:=new Calculator()
    fmt.Println(calc.Summary())
    fmt.Println(calc.Add(9,1))
}`
	generated, err := Compile("methods.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/methods\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "methods_goop.go"), generated, 0644)
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s\n%s", err, out, generated)
	}
	if string(out) != "10\n10\n" {
		t.Fatalf("got %q", out)
	}
}
