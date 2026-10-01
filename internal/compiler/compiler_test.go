package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const animals = `package main
import "fmt"

interface Speaker { Speak() string; }

abstract class Animal {
    private name string
    constructor(name string) { this.name = name }
    abstract Speak() string;
    Describe() string { return this.name + " says " + this.Speak() }
}
class Dog extends Animal implements Speaker {
    constructor(name string) { super(name) }
    override Speak() string { return "woof" }
}
class Cat extends Animal implements Speaker {
    constructor(name string) { super(name) }
    override Speak() string { return "meow" }
}
func main() {
    dog := new Dog("Rex")
    cat := new Cat("Milo")
    fmt.Println(dog.Describe())
    fmt.Println(cat.Describe())
}
`

func TestVirtualDispatchAndIntegration(t *testing.T) {
	generated, err := Compile("animals.goop", []byte(animals))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "self.__goopSelf.Speak()") {
		t.Fatal("virtual call not generated")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.org/test\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "animals_goop.go"), generated, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled Go failed: %v\n%s", err, output)
	}
	if string(output) != "Rex says woof\nMilo says meow\n" {
		t.Fatalf("unexpected output %q", output)
	}
}
func TestRewriteSkipsCommentsAndStrings(t *testing.T) {
	input := `package main
import "fmt"
class Dog {
  private name string
  constructor(s string) { this.name = s }
  Get() string {
    // this.name and new Dog("ignored")
    fmt.Println("this.name and new Dog(ignored)")
    return this.name
  }
}
func main() { fmt.Println(new Dog("ok").Get()) }
`
	got, err := Compile("quoted.goop", []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`// this.name and new Dog("ignored")`, `"this.name and new Dog(ignored)"`, `return self.name`, `NewDog("ok").Get()`} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("missing %q in\n%s", want, got)
		}
	}
}
func TestValidationErrors(t *testing.T) {
	cases := map[string]string{
		"missing superclass": `package main
class Dog extends Animal { }`,
		"duplicate member": `package main
class Dog { Run() {} Run() {} }`,
		"abstract method in concrete class": `package main
class Dog { abstract Speak() string; }`,
		"concrete subclass must override": `package main
abstract class A { abstract Speak() string; }
class B extends A { }`,
		"invalid override": `package main
class A { Run() {} }
class B extends A { override Speak() {} }`,
		"inheritance cycle": `package main
class A extends B {}
class B extends A {}`,
		"abstract instantiation": `package main
abstract class A { abstract Speak() string; }
func main() { _ = new A() }`,
		"super without extends": `package main
class A { constructor() { super() } }`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(name, []byte(input)); err == nil {
				t.Fatal("expected compilation error")
			}
		})
	}
}
func TestDefaultConstructorAndStaticMethod(t *testing.T) {
	input := `package main
class Base {
   virtual Value() string { return "base" }
}
class Child extends Base {
   override Value() string { return "child" }
   static Kind() string { return "Child" }
}
func main() { _ = new Child(); _ = Child_Kind() }`
	generated, err := Compile("default.goop", []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"func NewChild() *Child", "self.Base = *NewBase()", "func Child_Kind() string"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("expected %q", want)
		}
	}
}
func TestStaticFieldBecomesPackageVar(t *testing.T) {
	input := `package main
class Counter {
   static count int = 0
   private label string
   constructor(label string) { this.label = label }
}
func main() { _ = new Counter("a"); _ = Counter_count }`
	generated, err := Compile("static.goop", []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	got := string(generated)
	for _, want := range []string{"var Counter_count int = 0", "label string"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
	// A static field must NOT appear as a struct field or per-instance assignment.
	if strings.Contains(got, "count int\n") || strings.Contains(got, "self.count =") {
		t.Fatalf("static field leaked into the struct/constructor:\n%s", got)
	}
}
