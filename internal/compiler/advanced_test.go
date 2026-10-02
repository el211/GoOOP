package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageInheritanceGenericsInitializersAnnotations(t *testing.T) {
	sources := map[string][]byte{
		"animal.goop": []byte(`package main
@Reflect("root")
abstract class Animal[T any] {
    @Field("label")
    private label string = "animal"
    protected value T
    constructor(value T) { this.value = value }
    abstract Speak() string;
    Describe() string { return this.label + ":" + this.Speak() }
}`),
		"dog.goop": []byte(`package main
import "fmt"
@Serializable
class Dog extends Animal[string] {
    private count int = 3
    constructor(s string) { super(s) }
    @Override
    override Speak() string { return fmt.Sprintf("dog%d", this.count) }
}
func main() {
    d := new Dog("rex")
    fmt.Println(d.Describe())
    fmt.Println(GoOOPMetadataDog["annotations"])
}`),
	}
	outputs, err := CompileFiles(sources)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(outputs["animal.goop"]), "type Animal[T any] struct") {
		t.Fatalf("generic type not generated:\n%s", outputs["animal.goop"])
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/advanced\n\ngo 1.22\n"), 0644); err != nil {
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
	if !strings.Contains(string(result), "animal:dog3") || !strings.Contains(string(result), "Serializable") {
		t.Fatalf("unexpected output %q", result)
	}
}

func TestPrivateNestedClassIsUnexportedAndRuns(t *testing.T) {
	src := `package main
import "fmt"
class Outer {
    private class Secret {
        private v int
        constructor(v int) { this.v = v }
        Reveal() int { return this.v }
    }
    Make() int { return new Outer.Secret(9).Reveal() }
}
func main() { fmt.Println(new Outer().Make()) }`
	out, err := Compile("secret.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{"type outer_Secret struct", "func newOuter_Secret(", "newOuter_Secret(9)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
	// The private class must NOT leak as exported Go symbols.
	for _, bad := range []string{"type Outer_Secret struct", "func NewOuter_Secret", "GoOOPMetadataOuter_Secret"} {
		if strings.Contains(got, bad) {
			t.Fatalf("private class leaked exported symbol %q:\n%s", bad, got)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/secret\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret_goop.go"), out, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code failed: %v\n%s", err, result)
	}
	if strings.TrimSpace(string(result)) != "9" {
		t.Fatalf("unexpected output %q", result)
	}
}

func TestNestedClassDotAccessRunsEndToEnd(t *testing.T) {
	src := `package main
import "fmt"
class Outer {
    class Inner {
        private v int
        constructor(v int) { this.v = v }
        Get() int { return this.v }
    }
}
func main() {
    var box Outer.Inner = *new Outer.Inner(7)
    fmt.Println(box.Get())
}`
	out, err := Compile("nested.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{"var box Outer_Inner = *NewOuter_Inner(7)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/nested\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested_goop.go"), out, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code failed: %v\n%s", err, result)
	}
	if strings.TrimSpace(string(result)) != "7" {
		t.Fatalf("unexpected output %q", result)
	}
}

func TestPropertyGeneratesAccessorsEndToEnd(t *testing.T) {
	src := `package main
import "fmt"
class Person {
    property name string = "anon"
    constructor(name string) { this.name = name }
}
func main() {
    p := new Person("Ada")
    fmt.Println(p.GetName())
    p.SetName("Grace")
    fmt.Println(p.GetName())
}`
	out, err := Compile("person.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{"func (self *Person) GetName() string", "func (self *Person) SetName(value string)", "name string"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/prop\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "person_goop.go"), out, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code failed: %v\n%s", err, result)
	}
	if !strings.Contains(string(result), "Ada\nGrace") {
		t.Fatalf("unexpected output %q", result)
	}
}

func TestEnumDotAccessRunsEndToEnd(t *testing.T) {
	src := `package main
import "fmt"
enum Color { Red, Green, Blue }
func main() {
    c := Color.Green
    fmt.Println(int(c), c.String(), Color.Blue)
}`
	out, err := Compile("color.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "c := Color_Green") {
		t.Fatalf("enum dot access not rewritten:\n%s", out)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/enum\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "color_goop.go"), out, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code failed: %v\n%s", err, result)
	}
	if !strings.Contains(string(result), "1 Green Blue") {
		t.Fatalf("unexpected output %q", result)
	}
}

func TestStaticMemberDotAccess(t *testing.T) {
	src := `package main
import "fmt"
class Counter {
    static total int = 0
    static Bump(n int) int { Counter.total = Counter.total + n; return Counter.total }
}
func main() {
    fmt.Println(Counter.Bump(2))
    fmt.Println(Counter.total)
}`
	out, err := Compile("counter.goop", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{"var Counter_total int = 0", "func Counter_Bump(n int) int", "Counter_total = Counter_total + n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/static\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "counter_goop.go"), out, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code failed: %v\n%s", err, result)
	}
	if !strings.Contains(string(result), "2\n2") {
		t.Fatalf("unexpected output %q", result)
	}
}

func TestOverrideAllowsParameterRename(t *testing.T) {
	src := `package main
class Parent { virtual Sum(first int, second int) int { return first+second } }
class Child extends Parent { override Sum(a int,b int) int { return a+b } }
func main(){ _ = new Child().Sum(1,2) }`
	if _, err := Compile("rename.goop", []byte(src)); err != nil {
		t.Fatal(err)
	}
}

func TestGenericOverrideSpecialization(t *testing.T) {
	src := `package main
abstract class Base[T any] { abstract Value() T; }
class Child extends Base[string] { override Value() string {return "ok"} }
func main(){_ = new Child().Value()}`
	if _, err := Compile("special.goop", []byte(src)); err != nil {
		t.Fatal(err)
	}
}

func TestSubclassCannotAccessPrivateMembers(t *testing.T) {
	src := `package main
class Parent { private secret string = "hidden" }
class Child extends Parent { Read() string {return this.secret} }`
	_, err := Compile("privacy.goop", []byte(src))
	if err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("expected private access rejection: %v", err)
	}
}
