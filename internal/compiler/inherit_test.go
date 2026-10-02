package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runPkg(t *testing.T, mod string, files map[string]string) string {
	t.Helper()
	srcs := map[string][]byte{}
	for n, c := range files {
		srcs[n] = []byte(c)
	}
	outs, err := CompileFiles(srcs)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/"+mod+"\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for n, out := range outs {
		if err := os.WriteFile(filepath.Join(dir, n[:len(n)-5]+"_goop.go"), out, 0644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	res, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, res)
	}
	return string(res)
}

func TestInheritedOverloadNotHidden(t *testing.T) {
	// Child declares Show(string); it must NOT hide the inherited Show(int).
	out := runPkg(t, "inhov", map[string]string{"m.goop": `package main
import "fmt"
public class Base {
    constructor() {}
    public Show(v int) string { return fmt.Sprintf("int:%d", v) }
}
public class Child extends Base {
    constructor() { super() }
    public Show(v string) string { return "str:" + v }
}
func main() {
    c := new Child()
    fmt.Println(c.Show(7))
    fmt.Println(c.Show("hi"))
}`})
	if out != "int:7\nstr:hi\n" {
		t.Fatalf("got %q", out)
	}
}

func TestMultiLevelInheritedOverload(t *testing.T) {
	out := runPkg(t, "multilvl", map[string]string{"m.goop": `package main
import "fmt"
public class A { constructor() {} public F(a int) string { return "a" } }
public class B extends A { constructor() { super() } public F(a string) string { return "b" } }
public class C extends B { constructor() { super() } public F(a bool) string { return "c" } }
func main() {
    x := new C()
    fmt.Println(x.F(1) + x.F("y") + x.F(true))
}`})
	if out != "abc\n" {
		t.Fatalf("got %q", out)
	}
}

func TestCovariantReturnNonVirtual(t *testing.T) {
	out := runPkg(t, "covnv", map[string]string{"m.goop": `package main
import "fmt"
public class Animal { constructor() {} Kind() string { return "animal" } }
public class Dog extends Animal { constructor() { super() } override Kind() string { return "dog" } }
public class Factory {
    constructor() {}
    Create() *Animal { return new Animal() }
}
public class DogFactory extends Factory {
    constructor() { super() }
    override Create() *Dog { return new Dog() }
}
func main() {
    df := new DogFactory()
    fmt.Println(df.Create().Kind())
    var f *Factory = new Factory()
    fmt.Println(f.Create().Kind())
}`})
	if out != "dog\nanimal\n" {
		t.Fatalf("got %q", out)
	}
}

func TestInvalidOverrideReturnRejected(t *testing.T) {
	src := `package main
public class A { constructor() {} Get() int { return 1 } }
public class B extends A { constructor() { super() } override Get() string { return "x" } }
func main() { _ = new B() }`
	_, err := Compile("bad.goop", []byte(src))
	if err == nil || !contains(err.Error(), "covariant subtype") {
		t.Fatalf("expected invalid-override diagnostic, got: %v", err)
	}
}

func TestInvalidOverrideParamsRejected(t *testing.T) {
	src := `package main
public class A { constructor() {} virtual Do(x int) int { return x } }
public class B extends A { constructor() { super() } override Do(x string) int { return 0 } }
func main() { _ = new B() }`
	_, err := Compile("badp.goop", []byte(src))
	if err == nil || !contains(err.Error(), "override parameters differ") {
		t.Fatalf("expected param-mismatch diagnostic, got: %v", err)
	}
}

func TestVirtualCovariantReturnDiagnostic(t *testing.T) {
	src := `package main
public class Animal { constructor() {} }
public class Dog extends Animal { constructor() { super() } }
public class Factory { constructor() {} virtual Make() *Animal { return new Animal() } }
public class DogFactory extends Factory { constructor() { super() } override Make() *Dog { return new Dog() } }
func main() { _ = new DogFactory() }`
	_, err := Compile("vcov.goop", []byte(src))
	if err == nil || !contains(err.Error(), "covariant return type on a virtual") {
		t.Fatalf("expected virtual-covariant diagnostic, got: %v", err)
	}
}

func TestMultipleInheritanceMergedOverloads(t *testing.T) {
	// Both inherits Do(int) from Left and Do(string) from Right; the merged
	// overload set must expose both without Go embedding ambiguity.
	out := runPkg(t, "multinh", map[string]string{"m.goop": `package main
import "fmt"
public class P { constructor() {} }
public class Left extends P { constructor() { super() } public Do(a int) string { return fmt.Sprintf("L%d", a) } }
public class Right extends P { constructor() { super() } public Do(a string) string { return "R" + a } }
public class Both extends Left, Right {
    constructor() { super(); super.Right() }
}
func main() {
    b := new Both()
    fmt.Println(b.Do(1) + b.Do("x"))
}`})
	if out != "L1Rx\n" {
		t.Fatalf("got %q", out)
	}
}

func TestAmbiguousInheritedMethodDiagnostic(t *testing.T) {
	// Two parents promote the same method name/signature; GoOOP requires an
	// explicit override rather than relying on Go embedding resolution.
	src := `package main
public class L { constructor() {} Do() string { return "l" } }
public class R { constructor() {} Do() string { return "r" } }
public class D extends L, R { constructor() { super(); super.R() } }
func main() { _ = new D() }`
	_, err := Compile("amb.goop", []byte(src))
	if err == nil || !contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity diagnostic, got: %v", err)
	}
}
